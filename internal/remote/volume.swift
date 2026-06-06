// Gets/sets the BUILT-IN output device's volume (0-100). We target the built-in
// device directly because while AirTone streams, the default output is the
// "AirTone Sync" aggregate, which has no master volume (osascript returns
// "missing value"). The built-in speakers are the audible leg of that aggregate.
//
// Usage:  volume get        -> prints 0-100
//         volume set <0-100> -> sets it
import CoreAudio
import Foundation

func devices() -> [AudioDeviceID] {
    var size = UInt32(0)
    var addr = AudioObjectPropertyAddress(
        mSelector: kAudioHardwarePropertyDevices,
        mScope: kAudioObjectPropertyScopeGlobal,
        mElement: kAudioObjectPropertyElementMain)
    AudioObjectGetPropertyDataSize(AudioObjectID(kAudioObjectSystemObject), &addr, 0, nil, &size)
    let n = Int(size) / MemoryLayout<AudioDeviceID>.size
    var ids = [AudioDeviceID](repeating: 0, count: n)
    AudioObjectGetPropertyData(AudioObjectID(kAudioObjectSystemObject), &addr, 0, nil, &size, &ids)
    return ids
}

func transport(_ id: AudioDeviceID) -> UInt32 {
    var addr = AudioObjectPropertyAddress(mSelector: kAudioDevicePropertyTransportType,
        mScope: kAudioObjectPropertyScopeGlobal, mElement: kAudioObjectPropertyElementMain)
    var tt = UInt32(0); var size = UInt32(4)
    AudioObjectGetPropertyData(id, &addr, 0, nil, &size, &tt)
    return tt
}

func hasOutput(_ id: AudioDeviceID) -> Bool {
    var addr = AudioObjectPropertyAddress(mSelector: kAudioDevicePropertyStreams,
        mScope: kAudioObjectPropertyScopeOutput, mElement: kAudioObjectPropertyElementMain)
    var size = UInt32(0)
    AudioObjectGetPropertyDataSize(id, &addr, 0, nil, &size)
    return size > 0
}

func builtInOutput() -> AudioDeviceID? {
    for d in devices() where transport(d) == kAudioDeviceTransportTypeBuiltIn && hasOutput(d) {
        return d
    }
    return nil
}

// Volume scalar lives either on the main element or per-channel (1,2).
func volumeElements(_ dev: AudioDeviceID) -> [UInt32] { [kAudioObjectPropertyElementMain, 1, 2] }

func getVolume(_ dev: AudioDeviceID) -> Float32? {
    for el in volumeElements(dev) {
        var addr = AudioObjectPropertyAddress(mSelector: kAudioDevicePropertyVolumeScalar,
            mScope: kAudioObjectPropertyScopeOutput, mElement: el)
        if AudioObjectHasProperty(dev, &addr) {
            var v = Float32(0); var size = UInt32(MemoryLayout<Float32>.size)
            if AudioObjectGetPropertyData(dev, &addr, 0, nil, &size, &v) == noErr { return v }
        }
    }
    return nil
}

func setVolume(_ dev: AudioDeviceID, _ value: Float32) -> Bool {
    var ok = false
    for el in volumeElements(dev) {
        var addr = AudioObjectPropertyAddress(mSelector: kAudioDevicePropertyVolumeScalar,
            mScope: kAudioObjectPropertyScopeOutput, mElement: el)
        var settable = DarwinBoolean(false)
        if AudioObjectHasProperty(dev, &addr),
           AudioObjectIsPropertySettable(dev, &addr, &settable) == noErr, settable.boolValue {
            var v = value
            if AudioObjectSetPropertyData(dev, &addr, 0, nil, UInt32(MemoryLayout<Float32>.size), &v) == noErr {
                ok = true
            }
        }
    }
    return ok
}

let args = CommandLine.arguments
guard args.count >= 2, let dev = builtInOutput() else {
    FileHandle.standardError.write("error: no built-in output / bad args\n".data(using: .utf8)!)
    exit(1)
}

switch args[1] {
case "get":
    guard let v = getVolume(dev) else { print(0); exit(0) }
    print(Int((v * 100).rounded()))
case "set":
    guard args.count >= 3, let pct = Int(args[2]) else { exit(1) }
    let clamped = max(0, min(100, pct))
    exit(setVolume(dev, Float32(clamped) / 100.0) ? 0 : 1)
default:
    exit(1)
}
