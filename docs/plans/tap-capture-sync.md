---
topic: tap-capture-sync
state: active
phase: 2/4
next: kulak testi — Mac + iPhone aynı anda çalıyor mu (tek açık kapı)
updated: 2026-08-27
---

## status
phase 2/4 · kod tamam, **tek açık kapı kulak testi** · blocker: yok
Motor, Go yüzeyi ve dokümanlar bitti; BlackHole/sox/SwitchAudioSource bağımlılıkları silindi.
Boru hattı şu an CANLI (`airtone party`) — kullanıcı telefonla test edebilir.

## phases
- [x] 1 · Tap capture aracı            gate: ✅ probe `48000:16:2` · 931840 bayt/4.85 sn · RMS 0.247 (=0.35/√2, teorik tam) · 5 çıkış yolunun 5'i temiz
- [~] 2 · Party modu yeniden kablolama gate: ⏳ kulak testi bekliyor — kod ve ölçüm yeşil
      - [x] 2.1 start/stop/common      gate: ✅ boru hattı ayakta, 3 süreç, stream `playing`
      - [x] 2.2 başlangıç kilidi       gate: ✅ sessizlik pompası — snapclient kaydoldu, dışlama kuruldu
      - [x] 2.3 uçtan uca kanıt        gate: ✅ kaynak dışlanmışken RMS 0.2477 + FFT 440.0 Hz = snapclient geri çalıyor
      - [ ] 2.4 kulak testi            gate: Mac + iPhone aynı anda, 60 sn kesintisiz
- [x] 3 · setup/doctor/TUI yüzeyi      gate: ✅ `airtone doctor` 6/6 yeşil, BlackHole kurulu değilken
- [x] 4 · Instant modu + dokümanlar    gate: ✅ repo'da BlackHole yalnız tarihçe olarak geçiyor

## done
- 08-27: Faz 2-4 kapandı (kulak testi hariç). Motor tap'e geçti, Mac snapclient oldu, `sox`+`switchaudio-osx`+BlackHole bağımlılıkları ve cihaz geri-yükleme makinesi silindi. Kapılar: go vet/build/test + shellcheck yeşil; `airtone doctor` 6/6.
- 08-27: Uçtan uca kanıt — ton kaynağı DIŞLANMIŞken tap hâlâ RMS 0.2477 kaydetti ve FFT 440.0 Hz verdi; bu ses yalnız snapclient'in geri çalmasından gelebilir → zincir kapalı.
- 08-27: Başlangıç kilidi bulundu ve çözüldü — snapclient CoreAudio'ya ancak *çalarken* kaydoluyor, akış ise tap olmadan başlamıyor. Tap artık dışlanacak süreç belirene kadar gerçek-zamanlı sessizlik yayınlıyor.
- 08-27: Feasibility yeşil — probe 1: `frames=239616 peak=0.3500 rms=0.34944` (sentezlenen tonun tam genliği, cihaz değiştirilmeden). Probe 2 (eşli A/B, negatif hücre): dışlanmış `rms=0.00000` / dışlanmamış `rms=0.34874`, aynı frame sayısı.

## decisions
- Yakalama yolu = CoreAudio **process tap** (macOS 14.2+), BlackHole değil — sürücü kurulumu, admin şifresi, reboot ve Multi-Output cihazı tamamen ortadan kalkıyor; kullanıcının çıkış cihazı hiç değiştirilmiyor.
- Tap **kaynağında mute** eder (`CATapMutedWhenTapped`), Mac'in sesini **snapclient** geri çalar — senkronun tek sebebi bu: iki cihaz da artık snapcast istemcisi, aynı zaman ekseninde. (supersedes: 08-27 öncesi tasarım — Mac Multi-Output'tan 0 ms'de, telefon 1000 ms buffer'dan çalıyordu; kayma buradan geliyordu.)
- Geri-besleme koruması = tap **snapclient sürecini dışlar** (`initStereoGlobalTapButExcludeProcesses`). Bundle ID değil PID→AudioObjectID çevrimi, çünkü snapclient bundle'sız bir CLI. Ölçüldü, tutuyor.
- Sıralama sözleşmesi: snapserver → snapclient → (snapclient'in audio object'i kaydolana kadar poll) → tap. Tap yaratılırken dışlama listesi sabitlenir, sonradan değiştirilemez.
- macOS snapclient 0.35'te çıkış cihazı seçme bayrağı **yok** (`--help` doğrulandı; man sayfası bayat Linux sürümü). Sistem varsayılanına çalıyor — bu tasarımda sorun değil, varsayılan zaten kullanıcının hoparlörü.
- **Değişmez #2 yeniden tanımlanıyor:** "çıkışta ses cihazını geri yükle" → "çıkışta tap'i yok et (unmute)". Cihaz değiştirilmediği için geri yüklenecek bir şey yok; yeni yük taşıyan risk sızan tap = kalıcı sessiz Mac.
- `docs/troubleshooting.md`'deki "tap yolu TCC yüzünden CLI'da sessizlik döndürür, bu yüzden BlackHole şart" iddiası ÖLÇÜMLE ÇÜRÜDÜ ve geri çekildi — tap terminalden gerçek ses yakaladı. Doğru şerh: izin verilmemişse sessizlik gelir, çözüm izni vermek.
- Buffer 1000 → 500 ms — BlackHole roundtrip'i kalktığı için jitter düştü; Mac de artık buffer'dan çaldığından algılanan gecikmeyi yarıya indirmek istiyoruz. `AIRTONE_BUFFER` ile ayarlanabilir kalır.

## open / blockers
- open: kulak testi — Mac + iPhone gerçekten aynı anda mı? Tek kalan kapı; ölçüm zinciri kapattı ama sapmayı kulak yargılar.
- open: `CATapMutedWhenTapped` seçildi (çökmede ses kendiliğinden geri gelsin diye); `CATapMuted` ile farkı kulakla ayırt edilmedi — pratikte fark görülmedi.
- open: snapclient çökerse audio object ID değişir, dışlama kopar → geri-besleme. Şimdilik `ponytail:` şerhiyle bırakılıyor (gözlenmeden çözülmeyecek); gözlenirse tap'i yeniden yaratan bir watcher gerekir.
- open: iPhone Safari'de gerçek sapma bandı ölçülmedi — hedef ~50 ms, kulak testi faz 2 kapısı.

## non-goals
- Radyo/jukebox modu (YouTube linki verip çalma) — tap sistem sesini zaten aldığı için gereksizleşti.
- iPhone için native uygulama — "tarayıcı, uygulama yok" kilitli kararı duruyor; 10 ms bandı bu yüzden hedef değil.
- Linux/Windows desteği — ses yolu macOS-only değişmezi.
- L/R kanal ayırma (Mac sol, telefon sağ) — istenirse ayrı bir efor.

## why / scope
Kullanıcı Mac hoparlörü + iPhone'u tek bir senkron çift gibi kullanmak istiyor; mevcut
Party modunda Mac 0 ms'de, telefon ~1 s'de çaldığı için ikisi birlikte dinlenemiyor.
Kapsam: yakalamayı sürücüsüz tap'e taşımak, Mac'i de snapcast istemcisi yapmak ve
BlackHole/Multi-Output/cihaz-değiştirme zincirini (projenin en kırılgan parçası) silmek.

## refs
- research: `AudioHardwareTapping.h` + `CATapDescription.h` (macOS SDK) — tap API'sinin tek doğru kaynağı; `kAudioAggregateDeviceTapListKey` / `kAudioSubTapUIDKey` isimleri buradan doğrulandı
- research: feasibility probları — `scratchpad/tapprobe.swift` (yakalama), `scratchpad/tapexclude.swift` (eşli dışlama A/B)
- code:     `scripts/start.sh`            (boru hattının kurulduğu yer — sox+Multi-Output burada ölecek)
            `scripts/common.sh`           (tek-kaynak tunable'lar: BUFFER, CODEC, cihaz adları)
            `assets/systemtap.swift`      (YENİ — tap → stdout s16le; yakalamanın tamamı)
            `internal/instant/server.go`  (faz 4: `capturePipeline` sabiti hâlâ BlackHole'a bakıyor)
            `internal/doctor/doctor.go`   (faz 3: BlackHole/Multi-Output kontrolleri tap iznine dönecek)
