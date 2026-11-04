#!/usr/bin/env sh
# Uçtan uca duman testi: derlenmiş sunucuyu başlatır, kayıt/giriş/paylaşım
# akışını gerçek HTTP üzerinden dener. Kullanım: scripts/smoke.sh [port]
set -eu

PORT="${1:-8099}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
BASE="http://127.0.0.1:${PORT}"

cleanup() {
  [ -n "${SERVER_PID:-}" ] && kill "$SERVER_PID" 2>/dev/null || true
  rm -rf "$WORK"
}
trap cleanup EXIT

cd "$ROOT/services/api"
go build -o "$WORK/server" ./cmd/server
cd "$ROOT/apps/web"
[ -d out ] || npm run build

ADDR=":${PORT}" DB_PATH="$WORK/smoke.db" WEB_DIR="$ROOT/apps/web/out" "$WORK/server" &
SERVER_PID=$!

for _ in $(seq 1 50); do
  if curl -fsS "$BASE/api/health" >/dev/null 2>&1; then break; fi
  sleep 0.2
done
curl -fsS "$BASE/api/health" >/dev/null

echo "1/5 sağlık kontrolü tamam"

# Arayüzün kategorileri ve sıralamaları sunucudan gelir; hardcoded liste yok.
curl -fsS "$BASE/api/meta" | python3 -c 'import json,sys;d=json.load(sys.stdin);assert len(d["categories"])>=10;assert any(s["key"]=="yeni" for s in d["sorts"])'
echo "1b/5 meta kategorileri ve sıralamaları geldi"

START=$(curl -fsS -X POST "$BASE/api/auth/signup/start" \
  -H 'Content-Type: application/json' \
  -d '{"username":"Smoke_User"}')
SIGNUP_ID=$(printf '%s' "$START" | python3 -c 'import json,sys;print(json.load(sys.stdin)["signup_id"])')
SECRET=$(printf '%s' "$START" | python3 -c 'import json,sys;print(json.load(sys.stdin)["secret"])')
printf '%s' "$START" | python3 -c 'import json,sys;d=json.load(sys.stdin);assert d["qr"].startswith("data:image/png;base64,");assert d["otpauth_url"].startswith("otpauth://totp/")'
echo "2/5 kayıt başlatıldı (QR + secret geldi)"

CODE=$(python3 - "$SECRET" <<'PY'
import base64, hashlib, hmac, struct, sys, time
secret = sys.argv[1]
key = base64.b32decode(secret + "=" * ((8 - len(secret) % 8) % 8))
step = int(time.time()) // 30
digest = hmac.new(key, struct.pack(">Q", step), hashlib.sha1).digest()
offset = digest[-1] & 0x0F
value = struct.unpack(">I", digest[offset:offset + 4])[0] & 0x7FFFFFFF
print(f"{value % 1000000:06d}")
PY
)

curl -fsS -c "$WORK/jar" -X POST "$BASE/api/auth/signup/confirm" \
  -H 'Content-Type: application/json' \
  -d "{\"signup_id\":\"$SIGNUP_ID\",\"code\":\"$CODE\"}" > "$WORK/confirm.json"
python3 - "$WORK/confirm.json" <<'PY'
import json, sys
data = json.load(open(sys.argv[1]))
assert data["user"]["username"] == "smoke_user", data
assert len(data["recovery_codes"]) == 8, data
PY
RECOVERY=$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["recovery_codes"][0])' "$WORK/confirm.json")
echo "3/5 hesap açıldı (normalize ad + 8 kurtarma kodu)"

curl -fsS -b "$WORK/jar" "$BASE/api/auth/me" | grep -q '"smoke_user"'

curl -fsS -b "$WORK/jar" -X POST "$BASE/api/posts" \
  -H 'Content-Type: application/json' \
  -d '{"category":"Genel","title":"Duman testi paylaşımı","body":"Bu paylaşım duman testi tarafından oluşturuldu ve akışın baştan sona çalıştığını doğrular niteliktedir."}' \
  | grep -q '"author":"smoke_user"'
POST_ID=$(curl -fsS "$BASE/api/posts" | python3 -c 'import json,sys;print(json.load(sys.stdin)["posts"][0]["id"])')
curl -fsS "$BASE/api/posts/$POST_ID" | grep -q 'Duman testi paylaşımı'
echo "4/5 oturum + paylaşım + detay tamam"

curl -fsS -b "$WORK/jar" -X POST "$BASE/api/auth/logout" >/dev/null
STATUS=$(curl -s -o /dev/null -w '%{http_code}' -b "$WORK/jar" "$BASE/api/auth/me")
[ "$STATUS" = "401" ] || { echo "çıkış sonrası oturum hâlâ açık: $STATUS"; exit 1; }

LOGIN_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -c "$WORK/jar" -X POST "$BASE/api/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"smoke_user\",\"code\":\"$RECOVERY\"}")
[ "$LOGIN_STATUS" = "200" ] || { echo "kurtarma koduyla giriş başarısız: $LOGIN_STATUS"; exit 1; }

REUSE_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/api/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"smoke_user\",\"code\":\"$RECOVERY\"}")
[ "$REUSE_STATUS" = "401" ] || { echo "kurtarma kodu ikinci kez kabul edildi: $REUSE_STATUS"; exit 1; }

PAGE_STATUS=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/giris/")
[ "$PAGE_STATUS" = "200" ] || { echo "statik arayüz sunulamadı: $PAGE_STATUS"; exit 1; }
echo "5/6 çıkış, kurtarma kodu, tekrar kullanım engeli ve statik arayüz tamam"

# --- 2. kullanıcı: destek, kaydet, sohbet, bildirim ---
START2=$(curl -fsS -X POST "$BASE/api/auth/signup/start" \
  -H 'Content-Type: application/json' -d '{"username":"Okuyucu"}')
SIGNUP_ID2=$(printf '%s' "$START2" | python3 -c 'import json,sys;print(json.load(sys.stdin)["signup_id"])')
SECRET2=$(printf '%s' "$START2" | python3 -c 'import json,sys;print(json.load(sys.stdin)["secret"])')
CODE2=$(python3 - "$SECRET2" <<'PY'
import base64, hashlib, hmac, struct, sys, time
secret = sys.argv[1]
key = base64.b32decode(secret + "=" * ((8 - len(secret) % 8) % 8))
step = int(time.time()) // 30
digest = hmac.new(key, struct.pack(">Q", step), hashlib.sha1).digest()
offset = digest[-1] & 0x0F
value = struct.unpack(">I", digest[offset:offset + 4])[0] & 0x7FFFFFFF
print(f"{value % 1000000:06d}")
PY
)
curl -fsS -c "$WORK/jar2" -X POST "$BASE/api/auth/signup/confirm" \
  -H 'Content-Type: application/json' \
  -d "{\"signup_id\":\"$SIGNUP_ID2\",\"code\":\"$CODE2\"}" >/dev/null

curl -fsS -b "$WORK/jar2" -X POST "$BASE/api/posts/$POST_ID/support" | grep -q '"supported":true'
curl -fsS -b "$WORK/jar2" -X POST "$BASE/api/posts/$POST_ID/save" | grep -q '"saved":true'
curl -fsS -b "$WORK/jar2" "$BASE/api/me/saved" | grep -q 'Duman testi paylaşımı'

CONV=$(curl -fsS -b "$WORK/jar2" -X POST "$BASE/api/conversations" \
  -H 'Content-Type: application/json' \
  -d "{\"post_id\":\"$POST_ID\",\"message\":\"Merhaba, seni dinliyorum.\"}")
CONV_ID=$(printf '%s' "$CONV" | python3 -c 'import json,sys;print(json.load(sys.stdin)["conversation"]["id"])')
curl -fsS -b "$WORK/jar2" "$BASE/api/conversations" | grep -q '"peer_name":"smoke_user"'

# Yazar gelen kutunda okunmamış görür, cevap yazar, karşı tarafa bildirim düşer.
curl -fsS -b "$WORK/jar" "$BASE/api/summary" | grep -q '"unread_messages":1'
curl -fsS -b "$WORK/jar" -X POST "$BASE/api/conversations/$CONV_ID/messages" \
  -H 'Content-Type: application/json' -d '{"body":"Teşekkürler, iyi geldi."}' | grep -q '"sender_name":"smoke_user"'
curl -fsS -b "$WORK/jar2" "$BASE/api/notifications" | grep -q '"unread":1'

STATUS=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/api/me/stats")
[ "$STATUS" = "401" ] || { echo "çıkış yapmamış taraf stats aldı: $STATUS"; exit 1; }
curl -fsS -b "$WORK/jar2" "$BASE/api/me/stats" | grep -q '"recovery_codes_left":8'
echo "6/6 destek, kaydet, sohbet, bildirim ve istatistik tamam"
echo "duman testi başarılı"
