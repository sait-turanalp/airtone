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
- **`ffmpeg -f avfoundation` drops samples.** Replaced with callback-driven capture.
- **Multi-Output clock drift.** When BlackHole was a *non-master* sub-device, macOS
  resampled it and the capture timing wobbled → snapserver resynced several times a
  second → audible glitches. Fixed at the time by making BlackHole the master clock.

> **Superseded (2026-08).** Both of those belong to the old BlackHole + Multi-Output
> + sox path. Capture is now a **CoreAudio process tap**, which is driven by the
> device's own IO callback and has no second clock to drift against — the whole class
> of problem is gone, along with the driver install.

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
- **A tap that records pure silence** is a permissions problem, not a bug. macOS
  gates system-audio capture behind a one-time consent prompt attached to the app
  that launched the process — your terminal, or AirTone itself. If the tap runs but
  every sample is zero, grant that app audio/screen-recording permission in
  System Settings → Privacy & Security, then restart it.
  (An earlier version of this document claimed the tap path *always* returns zeros
  for CLI tools and that BlackHole was therefore unavoidable. That was measured to be
  wrong: `systemtap.swift` captures real audio from the terminal. The claim is
  retracted.)

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
  source clock (the tap) pace delivery; it's steadier than any Go pacer.

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

## "In Party mode the Mac itself goes quiet"

That is the design, not a fault. Party mode mutes the Mac's own output at the
source and plays the audio back through a local `snapclient` instead. That is the
only way the Mac can sit on the same clock as your phone — otherwise it runs a
full buffer ahead and the two can't be used together.

The mute lives exactly as long as the tap process, so `airtone stop`, quitting the
TUI, Ctrl+C, or even closing the terminal all bring the Mac's sound straight back.
If the Mac is ever left silent, the tap leaked — kill it:

```
pkill -f airtone-tap
```

Instant mode does **not** mute: there is no local player there, so the Mac keeps
playing normally and only the phone trails behind.

## Hard reset

```
airtone stop
pkill -x snapserver; pkill -x snapclient; pkill -f airtone-tap
rm -rf ~/.airtone
airtone setup
```
