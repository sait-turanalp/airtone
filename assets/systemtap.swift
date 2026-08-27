// AirTone system tap — captures macOS system audio with a CoreAudio process
// tap and writes raw interleaved s16le to stdout.
//
// Why a tap instead of a virtual driver: it needs no BlackHole install, no
// admin password, no reboot, and — the part that matters — it never touches the
// user's default output device. Nothing to switch, nothing to restore.
//
// The tap can MUTE the audio at its source (--mute). AirTone uses that so the
// Mac's own speakers stay silent and snapclient plays the stream back instead;
// that is what puts the Mac and the phone on the same clock. snapclient itself
// is excluded from the tap (--exclude-pid) or it would capture its own output
// in a feedback loop.
//
// Usage:
//   systemtap --probe                      print "<rate>:16:2" and exit
//   systemtap [--mute] [--exclude-pid N]…  stream s16le on stdout until killed
//
// Teardown is load-bearing: a leaked tap leaves the Mac permanently muted, so
// every exit path (SIGINT/SIGTERM/SIGHUP, closed stdout, fatal error) destroys
// the tap. Requires macOS 14.2+.
import AVFoundation
import CoreAudio
import Foundation

// MARK: - CLI

var excludedPIDs: [pid_t] = []
var muteAtSource = false
var probeOnly = false

var argv = Array(CommandLine.arguments.dropFirst())
while let arg = argv.first {
    argv.removeFirst()
    switch arg {
    case "--mute":
        muteAtSource = true
    case "--probe":
        probeOnly = true
    case "--exclude-pid":
        guard let raw = argv.first, let pid = pid_t(raw) else {
            FileHandle.standardError.write("systemtap: --exclude-pid needs a number\n".data(using: .utf8)!)
            exit(2)
        }
        argv.removeFirst()
        excludedPIDs.append(pid)
    default:
        FileHandle.standardError.write("systemtap: unknown argument \(arg)\n".data(using: .utf8)!)
        exit(2)
    }
}

// MARK: - CoreAudio helpers

let systemObject = AudioObjectID(kAudioObjectSystemObject)
var globalAddress = AudioObjectPropertyAddress(
    mSelector: 0,
    mScope: kAudioObjectPropertyScopeGlobal,
    mElement: kAudioObjectPropertyElementMain)

func log(_ msg: String) {
    FileHandle.standardError.write("systemtap: \(msg)\n".data(using: .utf8)!)
}

func bail(_ what: String, _ status: OSStatus) -> Never {
    teardown()
    log("\(what) failed (status \(status))")
    exit(1)
}

func defaultOutputUID() -> String? {
    var addr = globalAddress
    addr.mSelector = kAudioHardwarePropertyDefaultOutputDevice
    var device = AudioObjectID(0)
    var size = UInt32(MemoryLayout<AudioObjectID>.size)
    guard AudioObjectGetPropertyData(systemObject, &addr, 0, nil, &size, &device) == noErr else { return nil }

    addr.mSelector = kAudioDevicePropertyDeviceUID
    var uid: CFString = "" as CFString
    size = UInt32(MemoryLayout<CFString>.size)
    let status = withUnsafeMutablePointer(to: &uid) {
        AudioObjectGetPropertyData(device, &addr, 0, nil, &size, $0)
    }
    return status == noErr ? (uid as String) : nil
}

/// CoreAudio addresses processes by AudioObjectID, not PID. A process only gets
/// one once it actually touches audio, so this polls briefly for late starters
/// (snapclient needs a moment to open its output stream).
func processObject(pid: pid_t, waitFor timeout: TimeInterval = 5) -> AudioObjectID? {
    var addr = globalAddress
    addr.mSelector = kAudioHardwarePropertyTranslatePIDToProcessObject
    let deadline = Date().addingTimeInterval(timeout)
    repeat {
        var wanted = pid
        var object = AudioObjectID(kAudioObjectUnknown)
        var size = UInt32(MemoryLayout<AudioObjectID>.size)
        let status = AudioObjectGetPropertyData(
            systemObject, &addr, UInt32(MemoryLayout<pid_t>.size), &wanted, &size, &object)
        if status == noErr, object != kAudioObjectUnknown { return object }
        Thread.sleep(forTimeInterval: 0.1)
    } while Date() < deadline
    return nil
}

// MARK: - Teardown (idempotent, runs on every exit path)

var tapID = AudioObjectID(kAudioObjectUnknown)
var aggregateID = AudioObjectID(kAudioObjectUnknown)
var ioProcID: AudioDeviceIOProcID?
let teardownOnce = NSLock()
var toreDown = false

func teardown() {
    teardownOnce.lock()
    defer { teardownOnce.unlock() }
    if toreDown { return }
    toreDown = true

    if aggregateID != kAudioObjectUnknown, let proc = ioProcID {
        AudioDeviceStop(aggregateID, proc)
        AudioDeviceDestroyIOProcID(aggregateID, proc)
    }
    if aggregateID != kAudioObjectUnknown {
        AudioHardwareDestroyAggregateDevice(aggregateID)
    }
    // Destroying the tap is what un-mutes the system. Last and non-negotiable.
    if tapID != kAudioObjectUnknown {
        AudioHardwareDestroyProcessTap(tapID)
    }
}

// MARK: - Ring buffer (realtime callback -> writer thread)

/// The IO callback must never block on write(2), so it hands bytes to a writer
/// thread through a fixed ring. Overruns are counted, not silently swallowed.
/// ponytail: one lock guarding a memcpy; swap for a lock-free SPSC ring only if
/// overruns ever show up in the exit line.
final class Ring: @unchecked Sendable {
    private var buffer: [UInt8]
    private var head = 0, tail = 0, filled = 0
    private let condition = NSCondition()
    private(set) var overruns = 0
    private var closed = false

    init(capacity: Int) { buffer = [UInt8](repeating: 0, count: capacity) }

    func write(_ src: UnsafeRawPointer, _ count: Int) {
        condition.lock()
        defer { condition.unlock() }
        if buffer.count - filled < count {
            overruns += 1
            return
        }
        buffer.withUnsafeMutableBytes { dst in
            let first = min(count, buffer.count - head)
            memcpy(dst.baseAddress! + head, src, first)
            if first < count {
                memcpy(dst.baseAddress!, src + first, count - first)
            }
        }
        head = (head + count) % buffer.count
        filled += count
        condition.signal()
    }

    /// Blocks until bytes are available; returns nil once closed and drained.
    func read(into chunk: inout [UInt8]) -> Int? {
        condition.lock()
        defer { condition.unlock() }
        while filled == 0 && !closed { condition.wait() }
        if filled == 0 && closed { return nil }
        let count = min(filled, chunk.count)
        buffer.withUnsafeBytes { src in
            chunk.withUnsafeMutableBytes { dst in
                let first = min(count, buffer.count - tail)
                memcpy(dst.baseAddress!, src.baseAddress! + tail, first)
                if first < count {
                    memcpy(dst.baseAddress! + first, src.baseAddress!, count - first)
                }
            }
        }
        tail = (tail + count) % buffer.count
        filled -= count
        return count
    }

    func close() {
        condition.lock()
        closed = true
        condition.broadcast()
        condition.unlock()
    }
}

/// write(2) until the whole slice is gone. Returns false when stdout is gone
/// (EPIPE) — the signal that our parent died and we must un-mute and exit.
func writeAll(_ bytes: UnsafeRawPointer, _ count: Int) -> Bool {
    var offset = 0
    while offset < count {
        let n = write(1, bytes + offset, count - offset)
        if n > 0 {
            offset += n
            continue
        }
        if n < 0 && errno == EINTR { continue }
        return false
    }
    return true
}

// MARK: - Startup deadlock

/// A throwaway tap, purely to learn the device's format before we commit to one.
func probeFormat() -> AudioStreamBasicDescription {
    let probe = CATapDescription(stereoGlobalTapButExcludeProcesses: [])
    probe.name = "AirTone Tap Probe"
    probe.uuid = UUID()
    probe.isPrivate = true
    probe.muteBehavior = .unmuted

    var probeID = AudioObjectID(kAudioObjectUnknown)
    let created = AudioHardwareCreateProcessTap(probe, &probeID)
    if created != noErr { bail("create probe tap", created) }
    defer { AudioHardwareDestroyProcessTap(probeID) }

    var addr = globalAddress
    addr.mSelector = kAudioTapPropertyFormat
    var format = AudioStreamBasicDescription()
    var size = UInt32(MemoryLayout<AudioStreamBasicDescription>.size)
    let read = AudioObjectGetPropertyData(probeID, &addr, 0, nil, &size, &format)
    if read != noErr { bail("read probe format", read) }
    return format
}

/// snapclient only registers with CoreAudio once it starts playing, and it only
/// starts playing once the stream carries data — so the process we have to
/// exclude cannot exist until we are already streaming. Break the deadlock by
/// streaming real-time silence until it appears. The Mac stays quiet, which is
/// what --mute wants anyway, and snapserver sees a live stream throughout.
func resolveWhilePumpingSilence(
    _ pids: [pid_t], rate: Int, channels: Int, timeout: TimeInterval
) -> [AudioObjectID]? {
    let chunk = [UInt8](repeating: 0, count: (rate / 50) * channels * 2) // 20 ms
    let deadline = Date().addingTimeInterval(timeout)
    var nextTick = Date()
    while Date() < deadline {
        let resolved = pids.compactMap { processObject(pid: $0, waitFor: 0) }
        if resolved.count == pids.count { return resolved }
        guard chunk.withUnsafeBytes({ writeAll($0.baseAddress!, $0.count) }) else { return nil }
        nextTick = nextTick.addingTimeInterval(0.02)
        let sleepFor = nextTick.timeIntervalSinceNow
        if sleepFor > 0 { Thread.sleep(forTimeInterval: sleepFor) }
    }
    return nil
}

// MARK: - Build the tap

guard let outputUID = defaultOutputUID() else {
    log("no default output device")
    exit(1)
}

let deviceFormat = probeFormat()
let sampleRate = Int(deviceFormat.mSampleRate.rounded())
let channels = Int(deviceFormat.mChannelsPerFrame)

if probeOnly {
    print("\(sampleRate):16:\(channels)")
    exit(0)
}

var excludedObjects: [AudioObjectID] = []
if !excludedPIDs.isEmpty {
    log("waiting for \(excludedPIDs.count) process(es) to open audio (streaming silence meanwhile)")
    guard let resolved = resolveWhilePumpingSilence(
        excludedPIDs, rate: sampleRate, channels: channels, timeout: 30)
    else {
        log("excluded process never opened audio — refusing to run without the exclusion (feedback risk)")
        exit(1)
    }
    excludedObjects = resolved
}

let description = CATapDescription(stereoGlobalTapButExcludeProcesses: excludedObjects)
description.name = "AirTone Tap"
description.uuid = UUID()
description.isPrivate = true
description.muteBehavior = muteAtSource ? .mutedWhenTapped : .unmuted

var status = AudioHardwareCreateProcessTap(description, &tapID)
if status != noErr { bail("create process tap", status) }

var formatAddress = globalAddress
formatAddress.mSelector = kAudioTapPropertyFormat
var asbd = AudioStreamBasicDescription()
var asbdSize = UInt32(MemoryLayout<AudioStreamBasicDescription>.size)
status = AudioObjectGetPropertyData(tapID, &formatAddress, 0, nil, &asbdSize, &asbd)
if status != noErr { bail("read tap format", status) }

// The stream we already announced to snapserver cannot change shape mid-flight.
if Int(asbd.mSampleRate.rounded()) != sampleRate || Int(asbd.mChannelsPerFrame) != channels {
    bail("device format changed while starting up (was \(sampleRate):16:\(channels))", 0)
}

guard asbd.mFormatFlags & kAudioFormatFlagIsFloat != 0 else {
    bail("tap format is not float32 (got flags \(asbd.mFormatFlags))", 0)
}
let nonInterleaved = asbd.mFormatFlags & kAudioFormatFlagIsNonInterleaved != 0

let aggregateDescription: [String: Any] = [
    kAudioAggregateDeviceNameKey: "AirTone Tap Aggregate",
    kAudioAggregateDeviceUIDKey: UUID().uuidString,
    kAudioAggregateDeviceMainSubDeviceKey: outputUID,
    kAudioAggregateDeviceIsPrivateKey: true,
    kAudioAggregateDeviceIsStackedKey: false,
    kAudioAggregateDeviceTapAutoStartKey: true,
    kAudioAggregateDeviceSubDeviceListKey: [[kAudioSubDeviceUIDKey: outputUID]],
    kAudioAggregateDeviceTapListKey: [[
        kAudioSubTapDriftCompensationKey: true,
        kAudioSubTapUIDKey: description.uuid.uuidString,
    ]],
]
status = AudioHardwareCreateAggregateDevice(aggregateDescription as CFDictionary, &aggregateID)
if status != noErr { bail("create aggregate device", status) }

// MARK: - Capture

let ring = Ring(capacity: 1 << 19) // 512 KB ≈ 2.7 s at 48k/16/2
let stopped = DispatchSemaphore(value: 0)

status = AudioDeviceCreateIOProcIDWithBlock(&ioProcID, aggregateID, nil) { _, inputData, _, _, _ in
    let buffers = UnsafeMutableAudioBufferListPointer(UnsafeMutablePointer(mutating: inputData))
    guard let first = buffers.first, first.mData != nil else { return }

    if nonInterleaved {
        let frames = Int(first.mDataByteSize) / MemoryLayout<Float>.size
        var interleaved = [Int16](repeating: 0, count: frames * buffers.count)
        for (channel, buffer) in buffers.enumerated() {
            guard let raw = buffer.mData else { continue }
            let samples = raw.bindMemory(to: Float.self, capacity: frames)
            for frame in 0..<frames {
                interleaved[frame * buffers.count + channel] = toInt16(samples[frame])
            }
        }
        interleaved.withUnsafeBytes { ring.write($0.baseAddress!, $0.count) }
    } else {
        let count = Int(first.mDataByteSize) / MemoryLayout<Float>.size
        let samples = first.mData!.bindMemory(to: Float.self, capacity: count)
        var pcm = [Int16](repeating: 0, count: count)
        for i in 0..<count { pcm[i] = toInt16(samples[i]) }
        pcm.withUnsafeBytes { ring.write($0.baseAddress!, $0.count) }
    }
}
if status != noErr { bail("create IO proc", status) }

@inline(__always)
func toInt16(_ sample: Float) -> Int16 {
    let scaled = sample * 32767.0
    if scaled >= 32767 { return 32767 }
    if scaled <= -32768 { return -32768 }
    return Int16(scaled)
}

// Ignore SIGPIPE so a closed stdout surfaces as an EPIPE we can clean up after,
// rather than a signal that kills us with the tap still muting the Mac.
signal(SIGPIPE, SIG_IGN)
var signalSources: [DispatchSourceSignal] = []
for sig in [SIGINT, SIGTERM, SIGHUP] {
    signal(sig, SIG_IGN)
    let source = DispatchSource.makeSignalSource(signal: sig, queue: .global())
    source.setEventHandler { stopped.signal() }
    source.resume()
    signalSources.append(source)
}

status = AudioDeviceStart(aggregateID, ioProcID)
if status != noErr { bail("start capture", status) }

log("capturing \(sampleRate)Hz/\(channels)ch\(muteAtSource ? ", muted at source" : "")\(excludedObjects.isEmpty ? "" : ", excluding \(excludedPIDs.count) process(es)")")

let writer = Thread {
    var chunk = [UInt8](repeating: 0, count: 8192)
    while let n = ring.read(into: &chunk) {
        let ok = chunk.withUnsafeBytes { writeAll($0.baseAddress!, n) }
        if !ok {
            log("stdout closed, stopping")
            stopped.signal()
            return
        }
    }
}
writer.start()

stopped.wait()
AudioDeviceStop(aggregateID, ioProcID)
ring.close()
teardown()
if ring.overruns > 0 { log("\(ring.overruns) buffer overrun(s) — the reader could not keep up") }
log("stopped, audio un-muted")
exit(0)
