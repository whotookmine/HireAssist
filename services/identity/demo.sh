#!/bin/sh
# demo.sh — walks through member CRUD in order. Before each step it prints the call and waits for
# Enter; after each change it prints the members table from PostgreSQL.
#
#   make demo                 the service must already be running (`make run` in another terminal)
#   NO_PAUSE=1 sh demo.sh     run straight through without waiting

BASE=${BASE:-http://localhost:8081}
ADMIN_ID=00000000-0000-4000-8000-000000000001   # nok@acme.example, the only admin in the seed
JSON='Content-Type: application/json'
BODY=$(mktemp)
cd "$(dirname "$0")" || exit 1
trap 'rm -f "$BODY"' EXIT

bold=$(printf '\033[1m'); cyan=$(printf '\033[36m'); reset=$(printf '\033[0m')

# step "what this shows" METHOD PATH [JSON body] — prints the call, waits, sends it, prints the answer.
step() {
  title=$1 method=$2 path=$3 data=$4
  printf '\n%s%s> %s%s\n' "$bold" "$cyan" "$title" "$reset"
  if [ -n "$data" ]; then
    printf '  %s %s %s\n' "$method" "$path" "$data"
  else
    printf '  %s %s\n' "$method" "$path"
  fi
  if [ -z "$NO_PAUSE" ]; then printf '  [Enter to send]'; read -r _ </dev/tty; fi
  if [ -n "$data" ]; then
    status=$(curl -s -o "$BODY" -w '%{http_code}' -X "$method" "$BASE$path" -H "$JSON" -d "$data")
  else
    status=$(curl -s -o "$BODY" -w '%{http_code}' -X "$method" "$BASE$path")
  fi
  printf '  %s-> HTTP %s%s\n' "$bold" "$status" "$reset"
  if [ -s "$BODY" ]; then jq . "$BODY" 2>/dev/null || cat "$BODY"; fi
}

show_db() {
  printf '\n%s--- members table in PostgreSQL ---%s\n' "$bold" "$reset"
  make --no-print-directory db-show
}

printf '%sReset to the 2 demo members%s\n' "$bold" "$reset"
make --no-print-directory db-reset
show_db

step "READ ALL — the active members (200)" GET /members

step "CREATE — add Ploy as a recruiter with a temporary password (201)" POST /members \
  '{"email":"Ploy@Acme.example","password":"welcome-to-hireassist","role":"recruiter"}'
ID=$(jq -r .id "$BODY")
show_db

step "CREATE — invalid input lists every problem at once (400)" POST /members \
  '{"email":"not-an-email","password":"short","role":"manager"}'

step "CREATE — the same email again, in other capitals (409)" POST /members \
  '{"email":"ploy@acme.example","password":"another-long-password","role":"recruiter"}'

step "READ ONE — Ploy by id (200)" GET "/members/$ID"

step "UPDATE — make Ploy an admin (200)" PATCH "/members/$ID" '{"role":"admin"}'
show_db

step "UPDATE — give Ploy a new temporary password (200)" PUT "/members/$ID/password" \
  '{"password":"a-fresh-temporary-password"}'
show_db

step "DELETE — remove Ploy (204)" DELETE "/members/$ID"
show_db

step "READ ONE — Ploy is still there, marked removed (200)" GET "/members/$ID"
step "READ ALL — Ploy is no longer listed (200)" GET /members

step "DELETE — Nok is the last active admin (409)" DELETE "/members/$ADMIN_ID"
step "READ ONE — an id nobody has (404)" GET /members/11111111-2222-4333-8444-555555555555

printf '\n%sDone.%s\n' "$bold" "$reset"
