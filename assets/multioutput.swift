// Creates a macOS Multi-Output device named "AirTone Sync" that fans system
// audio to BOTH the built-in speakers (so the Mac stays audible) and BlackHole
// (so the audio can be captured). BlackHole is set as the master/clock device
// — this is what keeps the captured stream's timing stable (no constant resync).
//
// Device detection is generic (no language- or model-specific names):
//   - BlackHole: matched by name containing "blackhole"
//   - Speaker leg: the built-in output device (transport type "built-in"),
//                  falling back to the current default output device.
//
// Re-running is safe: an existing "AirTone Sync" is destroyed and recreated.
import CoreAudio
import Foundation

let kDeviceName = "AirTone Sync"
let kDeviceUID = "com.airtone.syncout"

func allDevices() -> [AudioDeviceID] {
    var size = UInt32(0)
    var addr = AudioObjectPropertyAddress(
        mSelector: kAudioHardwarePropertyDevices,
        mScope: kAudioObjectPropertyScopeGlobal,
        mElement: kAudioObjectPropertyElementMain)
    AudioObjectGetPropertyDataSize(AudioObjectID(kAudioObjectSystemObject), &addr, 0, nil, &size)
    let count = Int(size) / MemoryLayout<AudioDeviceID>.size
    var ids = [AudioDeviceID](repeating: 0, count: count)
    AudioObjectGetPropertyData(AudioObjectID(kAudioObjectSystemObject), &addr, 0, nil, &size, &ids)
    return ids
}

func stringProp(_ id: AudioDeviceID, _ selector: AudioObjectPropertySelector) -> String? {
    var addr = AudioObjectPropertyAddress(mSelector: selector,
        mScope: kAudioObjectPropertyScopeGlobal, mElement: kAudioObjectPropertyElementMain)
    var size = UInt32(MemoryLayout<CFString?>.size)
    var cf: CFString? = nil
    let st = withUnsafeMutablePointer(to: &cf) { ptr -> OSStatus in
        AudioObjectGetPropertyData(id, &addr, 0, nil, &size, ptr)
    }
    if st != noErr { return nil }
    return cf as String?
}

func uid(_ id: AudioDeviceID) -> String? { stringProp(id, kAudioDevicePropertyDeviceUID) }
func name(_ id: AudioDeviceID) -> String? { stringProp(id, kAudioObjectPropertyName) }

func transportType(_ id: AudioDeviceID) -> UInt32 {
    var addr = AudioObjectPropertyAddress(mSelector: kAudioDevicePropertyTransportType,
        mScope: kAudioObjectPropertyScopeGlobal, mElement: kAudioObjectPropertyElementMain)
    var tt = UInt32(0); var size = UInt32(MemoryLayout<UInt32>.size)
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

func defaultOutputUID() -> String? {
    var addr = AudioObjectPropertyAddress(mSelector: kAudioHardwarePropertyDefaultOutputDevice,
        mScope: kAudioObjectPropertyScopeGlobal, mElement: kAudioObjectPropertyElementMain)
    var dev = AudioDeviceID(0); var size = UInt32(MemoryLayout<AudioDeviceID>.size)
    AudioObjectGetPropertyData(AudioObjectID(kAudioObjectSystemObject), &addr, 0, nil, &size, &dev)
    return uid(dev)
}

// Destroy a previous "AirTone Sync" so re-runs are clean.
for d in allDevices() where uid(d) == kDeviceUID {
    AudioHardwareDestroyAggregateDevice(d)
}

// Locate BlackHole and the speaker leg.
var blackholeUID: String? = nil
var builtInUID: String? = nil
for d in allDevices() {
    guard let n = name(d)?.lowercased() else { continue }
    if n.contains("blackhole") { blackholeUID = uid(d) }
    if transportType(d) == kAudioDeviceTransportTypeBuiltIn && hasOutput(d) { builtInUID = uid(d) }
}

guard let bh = blackholeUID else {
    FileHandle.standardError.write("error: BlackHole device not found — install BlackHole 2ch\n".data(using: .utf8)!)
    exit(1)
}

// Prefer built-in speakers; otherwise use the current default output (unless it is BlackHole).
var speaker = builtInUID
if speaker == nil {
    if let def = defaultOutputUID(), def != bh, def != kDeviceUID { speaker = def }
}
guard let sp = speaker else {
    FileHandle.standardError.write("error: no speaker output device found\n".data(using: .utf8)!)
    exit(1)
}

let desc: [String: Any] = [
    kAudioAggregateDeviceNameKey as String: kDeviceName,
    kAudioAggregateDeviceUIDKey as String: kDeviceUID,
    kAudioAggregateDeviceIsStackedKey as String: 1,          // stacked => Multi-Output
    kAudioAggregateDeviceMasterSubDeviceKey as String: bh,   // BlackHole = master clock
    kAudioAggregateDeviceSubDeviceListKey as String: [
        [kAudioSubDeviceUIDKey as String: bh],
        [kAudioSubDeviceUIDKey as String: sp],
    ],
]

var newID = AudioDeviceID(0)
let status = AudioHardwareCreateAggregateDevice(desc as CFDictionary, &newID)
if status == noErr {
    print("created '\(kDeviceName)' (master=BlackHole, speaker=\(sp))")
} else {
    FileHandle.standardError.write("error: could not create device, OSStatus=\(status)\n".data(using: .utf8)!)
    exit(1)
}
