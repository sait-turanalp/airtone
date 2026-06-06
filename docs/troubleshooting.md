# Troubleshooting

This project exists because getting gapless, synced macOS audio onto a phone is
full of subtle traps. Here are the ones we hit, how to recognize them, and the
fixes — so you don't have to rediscover them.

Start with `airtone doctor`. It catches the common setup gaps (missing driver,
device, or Snapweb) before you debug audio.

---

## "It stutters every few seconds"

**The single most important lesson.** A small, periodic glitch is almost always
**source-timing drift**, not the network or the phone.

How we proved it (you can too):
1. Mac's own speakers sounded perfect → the source content was fine.
2. A synthetic, perfectly-paced source (a tone, then broadband pink noise) played
   on the phone **flawlessly** → the phone, Wi-Fi, codec, and buffer were all fine.
3. Capturing the real audio still stuttered → the **capture step** was dropping/
   mis-timing samples.

Two root causes and their fixes:
- **`ffmpeg -f avfoundation` drops samples.** Replaced with **`sox -t coreaudio`**,
  which is callback-driven and gapless.
- **Multi-Output clock drift.** When BlackHole is a *non-master* sub-device, macOS
  resamples it and the capture timing wobbles → snapserver resyncs several times a
  second → audible glitches. Fix: make **BlackHole the master/clock device** of the
  Multi-Output (AirTone's setup does this).

Quick diagnostic: watch the resync rate in the server log.
```
grep -c onResync ~/.airtone/snapserver.log    # should be ~0 in steady state
```

A pure tone can *mask* small gaps — if a constant tone sounds clean but music
stutters, suspect the capture, not the client.

---

## "The phone gets no sound / silence"

- Is something actually playing on the Mac, and is the output set to **AirTone Sync**?
  (`airtone start` sets it; `airtone status` should show the stream `playing`.)
- If you tried a Core Audio *tap*–based capturer (e.g. AudioTee) and got silence:
  that path needs `kTCCServiceAudioCapture` permission, which is **not** granted to
  CLI tools launched outside a TCC-aware terminal — it silently returns zeros. This
  is exactly why AirTone uses **BlackHole + sox** instead, which has no such wall.

## "The phone page is blank / shows a default page"

- The Snapweb player must be present at `~/.airtone/snapweb/index.html`. Run
  `airtone setup` to (re)download it. A "Snapcast Default Page" or "Placeholder"
  means the real player isn't being served.

## "Choppy even at a big buffer"

- If it stutters at a 2–4s buffer, it's **not** jitter (that much buffer absorbs
  jitter). Look at the capture/source instead (see the first section).
- Personal Hotspot is the worst case: the phone's Wi-Fi radio power-cycles and
  introduces gaps. Put both devices on a normal router if you can.

## "Mac went silent after stopping"

- `airtone stop` restores the previously selected output device. If it picked the
  wrong one, set the output back to your speakers in System Settings → Sound.

## Tuning latency vs smoothness

- `airtone` TUI → `c` (settings) → choose **Low / Balanced / Smooth**, or set
  `AIRTONE_BUFFER` (ms) before `airtone start`.
- Lower = snappier but needs a clean network; higher = rock-solid but more delay.

## Instant mode (WebRTC): why latency bottoms out on iOS

Instant mode uses WebRTC, so the receiver's adaptive jitter buffer (NetEQ)
dominates end-to-end latency. The page's live readout shows it:
`net … RTT · buffer … · ~… one-way`.

- **The network is rarely the problem.** On a good LAN, RTT is ~10-30ms.
- **The jitter buffer is the lever — but on iOS Safari it isn't adjustable.**
  We confirmed Safari ignores `playoutDelayHint` (Chrome-only), ignores
  `jitterBufferTarget` (20/40/80 all behaved the same), and does **not** advertise
  the `playout-delay` RTP header extension. Safari holds its own ~100ms floor, so
  total lands ~130ms — the realistic A-tier floor for browser-on-iOS.
- **Don't force it lower.** Aggressive settings (tiny target, a wall-clock pacer)
  make NetEQ oscillate (100-400ms) — worse than a stable ~130ms. Let the audio
  source clock (sox) pace delivery; it's steadier than any Go pacer.

To actually go lower:
- **Android Chrome** honors the buffer hints → typically ~30-60ms on the same LAN.
- A **native iOS client** (bypassing Safari's NetEQ) is the only way lower on Apple — a roadmap item.

For perfectly-synced multi-device audio, use **Party mode** instead — sync and low
latency are different goals.

## Remote control: a button "does nothing"

The control bar uses Apple's MediaRemote (via the bundled, permission-free
mediaremote-adapter). How well transport works depends on the **app's**
MediaRemote support:

- **Spotify / Apple Music** — reliable: play/pause and next/previous all work.
- **Browser media (YouTube, etc.)** — flaky: commands are sent (the bar gets a
  success), but the browser may apply them late or ignore them, and a single
  video has no "next/previous track" at all. play/pause may need a second tap.
- **Volume** always works (it's a direct CoreAudio call, not MediaRemote).

The ⏯ button shows ▶ or ⏸ to reflect the current state. If transport feels
unreliable, you're almost certainly controlling a browser tab — use a real
music app for solid behavior. This is the app's behavior, not an AirTone bug.

Volume targets the **built-in speakers** directly (the "AirTone Sync" aggregate
used while streaming has no master volume, so `osascript set volume` can't drive
it). If volume seems to do nothing, your audible output isn't the built-in
speakers.

## "The macOS volume keys / menu slider don't change the sound while streaming"

Expected — it's a macOS limitation, not an AirTone bug. While streaming, the
system output is the **AirTone Sync Multi-Output device**, and macOS Multi-Output
(and Aggregate) devices have **no master volume**: the volume keys grey out and
the menu slider is a no-op for them.

Control volume from the **AirTone app's slider** instead — it sets the built-in
speakers' volume directly via CoreAudio, so it always works. (The keys/menu work
again as soon as you switch back to a normal output, which AirTone does for you
on exit.)

## Hard reset

```
airtone stop
pkill -x snapserver; pkill -x sox
rm -rf ~/.airtone
airtone setup
```
