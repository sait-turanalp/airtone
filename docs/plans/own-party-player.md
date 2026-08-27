---
topic: own-party-player
state: active
phase: 1/4
next: 1.1 LICENSE MIT -> GPL-3.0
updated: 2026-08-27
---

## status
phase 1/4 · next: yeniden lisanslama · blocker: yok
Party modundaki telefon oynatıcısı Snapweb'den AirTone'un kendi arayüzüne taşınıyor.
CEO kararı alındı: tek repo, tek build, AirTone GPL-3.0'a geçiyor.

## phases
- [~] 1 · Yeniden lisanslama              gate: repoda MIT iddiası kalmaz; LICENSE·README·goreleaser·THIRD_PARTY tutarlı
- [ ] 2 · `web/` projesi + bundle          gate: `npm run build` tek self-contained çıktı üretir; tarayıcıda ses çalar
- [ ] 3 · Go ön kapı (1780)                gate: telefon `http://ip:1780`'de bizim arayüzü görür, senkron ses + now-playing çalışır
- [ ] 4 · Snapweb'i sök + dokümanlar       gate: setup Snapweb indirmez, doctor kontrolü kalkar, repoda snapweb referansı kalmaz

## done
- 08-27: Fizibilite — Snapweb'in ses motoru arayüzden AYRIK: `snapstream.ts` (42 KB, WebSocket+protokol+Opus/FLAC+zaman senkronu+WebAudio) ve `snapcontrol.ts` (18 KB, JSON-RPC). `components/*.tsx` sadece arayüz. Yani headless kullanım kaynak seviyesinde mümkün.

## decisions
- **AirTone MIT → GPL-3.0.** `snapstream.ts` repoya girdiği an türev eser oluyor; Snapweb GPL-3.0. CEO tek-yön kapıyı bilerek geçti (alternatifler: ayrı GPL repo forku · sıfırdan MIT istemci · sadece tema — üçü de reddedildi). Mevcut bağımlılıklar (pion MIT, bubbletea MIT, mediaremote BSD-3) GPL uyumlu, engel yok.
- **npm AirTone'un derleme yoluna GİRMEZ.** `web/` altında vite projesi durur, çıktısı `assets/player/`'a commit'lenir; `go build` tek başına çalışan binary üretmeye devam eder. Bedeli: oynatıcı güncellemesi elle bir komut (`cd web && npm run build`), CONTRIBUTING'e yazılacak.
- **Ön kapıyı AirTone'un Go sunucusu alır (1780), snapserver arkaya çekilir (1785).** Sebep: sayfa `remote.html` gibi görünecekse now-playing/kapak/kontroller lazım, onlar `/control/*` uçlarında ve snapserver onları servis edemez. Kullanıcının bildiği URL değişmez.
- **Ses WebSocket'i Go üzerinden proxy'lenir** (hijack + çift yönlü kopya, ~40 satır) — sayfa tek origin'de kalsın, CORS derdi ve URL'de ikinci port olmasın.
- **Party'de telefonun ses kaydırıcısı KENDİ snapcast istemci sesini ayarlar** (JSON-RPC `Client.SetVolume`), Mac'in sesini değil. Instant'ta tersi (Mac'in sesi) — aynı görünen kontrol iki modda farklı şeye bağlanır, karıştırılmasın.

## open / blockers
- open: `snapstream.ts` sürüm takibi — upstream düzeltmeleri elle alınacak; hangi commit'ten vendor'landığı `web/VENDOR.md`'ye yazılacak.
- open: iOS Safari'de wasm Opus decoder + WebAudio bellek davranışı ölçülmedi; Snapweb'in kendisi çalıştığına göre risk düşük ama bizim bundle'da doğrulanacak.
- open: Snapweb'in `10-seconds-of-silence.mp3` numarası (iOS'ta ses bağlamını canlı tutma hilesi) bizim sayfaya da lazım mı — faz 2'de görülecek.

## non-goals
- Instant modunun arayüzünü değiştirmek — `remote.html` tasarım referansı, kaynak olarak kalıyor.
- Snapweb'in çoklu-grup/çoklu-istemci yönetim ekranları — AirTone tek grup kullanıyor.
- Kendi senkron algoritmamızı yazmak — kanıtlanmış `snapstream.ts` motoru aynen korunuyor, değişen sadece arayüz.

## why / scope
Party modunda telefonda Snapweb açılıyor ve ürünün geri kalanının yanında eski duruyor;
Instant modundaki kendi oynatıcımız (`internal/remote/remote.html`) ise cilalı. Amaç iki
modda da aynı AirTone arayüzü. Snapweb'in ses motoru korunur, arayüzü atılır.

## refs
- research: `snapweb` deposu (GPL-3.0) — `src/snapstream.ts` + `src/snapcontrol.ts` = motor, `src/components/` = atılacak arayüz
- code:     `internal/remote/remote.html`  (tasarım referansı — Party oynatıcısı buna benzeyecek)
            `internal/remote/http.go`      (`/control/*` uçları — Party'de de bunlar servis edilecek)
            `assets/snapserver.conf.tmpl`  (doc_root + http port; snapserver 1785'e çekilecek)
            `scripts/common.sh`            (port tanımları tek kaynak)
            `web/`                         (YENİ — vite projesi, çıktısı assets/player/'a gider)
