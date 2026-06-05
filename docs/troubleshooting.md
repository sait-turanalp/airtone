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

## Hard reset

```
airtone stop
pkill -x snapserver; pkill -x sox
rm -rf ~/.airtone
airtone setup
```
