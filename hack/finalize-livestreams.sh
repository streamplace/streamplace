#!/usr/bin/env bash
# Turn a streamer's recorded livestreams into VODs, from a node.
#
# Reads the streamer's place.stream.livestream records from their PDS, groups
# the ones that belong to one broadcast (a client that starts a new record on
# a title change splits a recording across two records; records created on
# the same UTC day form one group unless --group says otherwise), titles each
# VOD from its first record (the leading emoji and a "Watch now:" prefix
# stripped), shows the plan, and on a yes runs the node's operator finalize
# for each group, publishing the video and ending the records the streamer
# never stopped. Then it waits for each finalize to finish and lists the
# streamer's videos with their track counts.
#
# Run on a node (the internal API is loopback). The node needs to be able to
# write for the streamer: --account-credentials / SP_ACCOUNT_CREDENTIALS, or a
# stored session. Needs curl and python3.
#
#   hack/finalize-livestreams.sh <handle-or-did> [options]
#
#   --group rkey1,rkey2   records to finalize as one VOD (repeatable; replaces
#                         the by-day grouping; records not named are skipped)
#   --title "..."         title for every group (only sensible with one group)
#   --skip-ended          leave out records that were ended (default: include)
#   --no-publish          finalize only, leave a draft; publish later with POST /videos
#   --keep-live           do not end records that were never stopped
#   --end-only            only end the never-stopped records; no VOD (for
#                         records left live after a finalize that already ran)
#   --yes                 no confirmation prompt
#   --node URL            internal API (default http://127.0.0.1:39090)
#   --wait MINUTES        how long to wait for each finalize (default 40)
set -euo pipefail

NODE="${SP_INTERNAL_API:-http://127.0.0.1:39090}"
WAIT_MINUTES=40
GROUP_ARGS=()
TITLE=""
SKIP_ENDED=0
PUBLISH=true
END_LIVE=true
END_ONLY=false
YES=0
WHO=""

while [ $# -gt 0 ]; do
  case "$1" in
    --group) GROUP_ARGS+=("$2"); shift 2 ;;
    --title) TITLE="$2"; shift 2 ;;
    --skip-ended) SKIP_ENDED=1; shift ;;
    --no-publish) PUBLISH=false; shift ;;
    --keep-live) END_LIVE=false; shift ;;
    --end-only) END_ONLY=true; shift ;;
    --yes|-y) YES=1; shift ;;
    --node) NODE="$2"; shift 2 ;;
    --wait) WAIT_MINUTES="$2"; shift 2 ;;
    -h|--help) sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    -*) echo "unknown option: $1" >&2; exit 2 ;;
    *) WHO="$1"; shift ;;
  esac
done
if [ -z "$WHO" ]; then
  echo "usage: $0 <handle-or-did> [--group rkey1,rkey2 ...] [--title ...] [--no-publish] [--keep-live] [--yes]" >&2
  exit 2
fi
for tool in curl python3; do
  command -v "$tool" >/dev/null || { echo "$tool is required" >&2; exit 2; }
done

# --- resolve the account and read its livestream records (public data) ---
plan_json=$(python3 - "$WHO" "$SKIP_ENDED" "$TITLE" "${GROUP_ARGS[@]:-}" <<'PY'
import json, sys, urllib.request, urllib.parse, datetime, re
who, skip_ended, title_override = sys.argv[1], sys.argv[2] == "1", sys.argv[3]
groups = [g for g in sys.argv[4:] if g]

def get(url):
    with urllib.request.urlopen(url, timeout=60) as r:
        return json.load(r)

# handle -> DID: the handle's own well-known file first (a network's handles
# may be unknown to Bluesky's resolver), then Bluesky's resolver.
did = who
if not who.startswith("did:"):
    try:
        with urllib.request.urlopen(f"https://{who}/.well-known/atproto-did", timeout=30) as r:
            did = r.read().decode().strip()
    except Exception:
        did = ""
    if not did.startswith("did:"):
        try:
            did = get("https://public.api.bsky.app/xrpc/com.atproto.identity.resolveHandle?handle=" + urllib.parse.quote(who))["did"]
        except Exception as e:
            sys.exit(f"could not resolve {who} to a DID ({e}); pass the DID instead")
if did.startswith("did:plc:"):
    doc = get("https://plc.directory/" + did)
else:
    host = did[len("did:web:"):]
    doc = get(f"https://{host}/.well-known/did.json")
pds = next(s["serviceEndpoint"] for s in doc.get("service", []) if s.get("id") == "#atproto_pds")
handle = next((a[len("at://"):] for a in doc.get("alsoKnownAs", []) if a.startswith("at://")), did)

recs, cursor = [], None
while True:
    q = {"repo": did, "collection": "place.stream.livestream", "limit": 100}
    if cursor:
        q["cursor"] = cursor
    out = get(pds + "/xrpc/com.atproto.repo.listRecords?" + urllib.parse.urlencode(q))
    recs += out.get("records", [])
    cursor = out.get("cursor")
    if not cursor or not out.get("records"):
        break

# a record's real creation instant is its TID key (createdAt is the client's word)
ALPHA = "234567abcdefghijklmnopqrstuvwxyz"
def tid_time(rkey):
    n = 0
    for c in rkey:
        n = n * 32 + ALPHA.index(c)
    return datetime.datetime.fromtimestamp((n >> 10) / 1e6, datetime.UTC)

def clean_title(t):
    # the first line only: a post-style title may carry a second sentence
    t = t.strip().splitlines()[0].strip() if t.strip() else ""
    # a leading emoji / symbol run, then an optional "Watch now:" / "Watch LIVE:" call to action
    t = re.sub(r"^[^\w(\[\"']+", "", t)
    t = re.sub(r"^(watch\s+(now|live)\s*[:\-–—]\s*)", "", t, flags=re.I)
    return t.strip() or "Livestream"

items = []
for r in recs:
    rkey = r["uri"].rsplit("/", 1)[1]
    v = r["value"]
    items.append({
        "uri": r["uri"], "rkey": rkey, "created": tid_time(rkey).isoformat(timespec="seconds"),
        "day": tid_time(rkey).strftime("%Y-%m-%d"), "title": v.get("title", ""), "ended": v.get("endedAt"),
    })
items.sort(key=lambda i: i["created"])
if skip_ended:
    items = [i for i in items if not i["ended"]]

if groups:
    by_rkey = {i["rkey"]: i for i in items}
    plan = []
    for g in groups:
        members = []
        for rk in g.split(","):
            rk = rk.strip()
            if rk not in by_rkey:
                sys.exit(f"record {rk} is not one of {handle}'s livestream records")
            members.append(by_rkey[rk])
        members.sort(key=lambda i: i["created"])
        plan.append(members)
else:
    days = {}
    for i in items:
        days.setdefault(i["day"], []).append(i)
    plan = [days[d] for d in sorted(days)]

out = {"did": did, "handle": handle, "pds": pds, "groups": []}
for members in plan:
    out["groups"].append({
        "title": title_override or clean_title(members[0]["title"]),
        "records": members,
    })
print(json.dumps(out))
PY
)

PLAN="$plan_json" python3 - <<'PY'
import json, os, sys
p = json.loads(os.environ["PLAN"])
print("Streamer: %s (%s) on %s" % (p["handle"], p["did"], p["pds"]))
if not p["groups"]:
    print("No livestream records to finalize."); sys.exit(1)
for n, g in enumerate(p["groups"], 1):
    print('\nVOD %d: "%s"' % (n, g["title"]))
    for r in g["records"]:
        state = "ended " + r["ended"][:19] if r["ended"] else "never ended"
        print("   %s  created %s  %s" % (r["rkey"], r["created"], state))
        print("      " + r["title"][:90])
PY
echo
if [ "$END_ONLY" = true ]; then echo "Node: $NODE   END ONLY: no VOD is made, the never-stopped records are ended"; else echo "Node: $NODE   publish: $PUBLISH   end never-stopped records: $END_LIVE"; fi
if [ "$YES" != "1" ]; then
  read -r -p "Finalize these? [y/N] " answer
  case "$answer" in y|Y|yes|YES) ;; *) echo "aborted"; exit 1 ;; esac
fi

did=$(PLAN="$plan_json" python3 -c 'import json,os; print(json.loads(os.environ["PLAN"])["did"])')
count=$(PLAN="$plan_json" python3 -c 'import json,os; print(len(json.loads(os.environ["PLAN"])["groups"]))')
upload_ids=()

for ((n=0; n<count; n++)); do
  body=$(PLAN="$plan_json" N="$n" PUBLISH="$PUBLISH" END_LIVE="$END_LIVE" END_ONLY="$END_ONLY" python3 - <<'PY'
import json, os
g = json.loads(os.environ["PLAN"])["groups"][int(os.environ["N"])]
if os.environ["END_ONLY"] == "true":
    print(json.dumps({"livestreams": [r["uri"] for r in g["records"]], "endOnly": True}))
else:
    print(json.dumps({"livestreams": [r["uri"] for r in g["records"]], "title": g["title"],
                      "publish": os.environ["PUBLISH"] == "true", "endLivestream": os.environ["END_LIVE"] == "true"}))
PY
)
  echo
  if [ "$END_ONLY" = true ]; then echo "=== group $((n+1)): ending records"; else echo "=== VOD $((n+1)): finalizing"; fi
  resp=$(curl -sS -X POST "$NODE/finalize-livestream" -H 'content-type: application/json' -d "$body")
  RESP="$resp" python3 - <<'PY' || exit 1
import json, os, sys
try:
    r = json.loads(os.environ["RESP"])
except ValueError:
    print("FAILED: not a JSON answer from the node:", os.environ["RESP"][:300]); sys.exit(1)
if "error" in r:
    print("FAILED:", r.get("error"), "-", r.get("error_detail", "")); sys.exit(1)
if r.get("uploadId"):
    print('   upload %s: %d recorded objects, %.2f GB, title "%s"' % (r["uploadId"], r["objects"], r["bytes"] / 1e9, r["title"]))
for u in r.get("ended", []): print("   ended record", u)
for e in r.get("endErrors", []): print("   could not end:", e)
PY
  if [ "$END_ONLY" != true ]; then
    upload_ids+=("$(RESP="$resp" python3 -c 'import json,os; print(json.loads(os.environ["RESP"])["uploadId"])')")
  fi
done
if [ "$END_ONLY" = true ]; then echo; echo "Done: records ended, nothing else changed."; exit 0; fi

# --- wait for the finalizes to finish (the node copies the recording in the background) ---
echo
echo "Waiting for the node to finish (up to $WAIT_MINUTES minutes each; the recording is copied server-side)..."
deadline=$(( $(date +%s) + WAIT_MINUTES * 60 ))
for id in "${upload_ids[@]}"; do
  while :; do
    status=$(curl -sS "$NODE/videos?repo=$did" | ID="$id" python3 -c '
import json, os, sys
v = json.load(sys.stdin)
u = next((u for u in v.get("uploads", []) if u["uploadId"] == os.environ["ID"]), None)
print((u or {}).get("status") or "unknown", (u or {}).get("error") or "")
')
    case "$status" in
      done*) echo "   upload $id: done"; break ;;
      error*) echo "   upload $id: FAILED: ${status#error }"; break ;;
    esac
    if [ "$(date +%s)" -gt "$deadline" ]; then echo "   upload $id: still '$status' after $WAIT_MINUTES minutes; check Loki for 'livestream VOD finalized'"; break; fi
    sleep 10
  done
done

echo
echo "=== $did videos now"
curl -sS "$NODE/videos?repo=$did" | python3 -c '
import json, sys
v = json.load(sys.stdin)
for x in v.get("videos", []):
    print("   %s\n      \"%s\"  %d tracks  %.1f min" % (x["uri"], x["title"], len(x["tracks"]), x["durationMs"] / 60000))
if not v.get("videos"): print("   (none; if publish was off, publish with POST /videos)")
'
echo
echo "Retitle with:  curl -X PUT $NODE/videos -H 'content-type: application/json' -d '{\"uri\":\"<video uri>\",\"title\":\"...\"}'"
