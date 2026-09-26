#!/usr/bin/env bash
# Fills an empty mikado server with the demo journeys used for development,
# testing and the README screenshots: goals with and without code.
#
#   MIKADO_SERVER=http://127.0.0.1:47295 src/demo/seed.sh [path/to/mikado]
#
# Run it against a fresh data directory (make demo does): the keys it makes
# (J1…J5, Q1…) depend on starting from nothing.
set -euo pipefail

m=${1:-mikado}
: "${MIKADO_SERVER:?set MIKADO_SERVER to the demo server, never to your real one}"
export MIKADO_SERVER

if [ -n "$("$m" journey list --all --json | jq 'length | select(. > 0)')" ]; then
	echo "seed.sh: $MIKADO_SERVER already has journeys; seed an empty data directory" >&2
	exit 1
fi

key() { jq -r .key; }
quiet() { "$@" >/dev/null; }

# J1: everything done, achievements too (the Atlas's 100% shelf).
quiet "$m" journey new "The home office moves into the spare room" 2>/dev/null
a=$("$m" errand "Carry the desk and chair into the spare room" --crowns J1 --hero erkan --json | key)
b=$("$m" errand "Clear the old boxes out of the spare room" --opens "$a" --hero erkan --json | key)
c=$("$m" errand "Run a network cable along the skirting board" --opens "$a" --hero erkan --json | key)
s=$("$m" errand "Hang the map of Istanbul above the desk" --side-of "$a" --json | key)
for q in "$b" "$c" "$a" "$s"; do quiet "$m" fulfil "$q"; done

# J2: main quest fulfilled, one achievement left.
quiet "$m" journey new "Touch-type at 60 words a minute" 2>/dev/null
a=$("$m" errand "Pass a 60 wpm test three days in a row" --crowns J2 --json | key)
b=$("$m" errand "Practise 15 minutes every morning for a month" --opens "$a" --json | key)
quiet "$m" errand "Reach 80 wpm on a code-typing test" --side-of "$a"
s=$("$m" errand "Learn the symbols row without looking" --side-of "$a" --json | key)
for q in "$b" "$a" "$s"; do quiet "$m" fulfil "$q"; done

# J3: an omelette — sealed, open, underway and fulfilled quests, a found
# quest, side quests on side quests and a petition.
quiet "$m" journey new "Make an omelette" 2>/dev/null
serve=$("$m" errand "Serve the omelette on a warm plate while it is still soft in the middle" --crowns J3 --json | key)
whisk=$("$m" errand "Whisk three eggs with a pinch of salt until no streaks of white remain" --opens "$serve" --json | key)
heat=$("$m" errand "Heat a knob of butter in the pan until it foams but doesn't brown" --opens "$serve" --json | key)
eggs=$("$m" errand "Make sure there are three eggs in the fridge" --opens "$whisk" --hero erkan --json | key)
salt=$("$m" errand "Make sure there is salt in the grinder or the jar" --opens "$whisk" --json | key)
pan=$("$m" errand "Wash and dry the 20 cm non-stick pan" --opens "$heat" --hero erkan --json | key)
butter=$("$m" errand "Take the butter out of the fridge and cut a 15 g knob" --opens "$heat" --json | key)
shop=$("$m" errand "Buy a box of eggs from the corner shop" --opens "$eggs" --found-on "$eggs" \
	--reason "only two eggs left, the recipe needs three" --hero erkan --json | key)
filling=$("$m" errand "Grate a handful of cheese and chop the chives for a filling" --side-of "$serve" --json | key)
quiet "$m" petition "Ask your flatmate whether the cheddar in the fridge is theirs" --on flatmate --side-of "$filling"
quiet "$m" errand "Grind black pepper into the eggs just before cooking" --side-of "$whisk"
quiet "$m" errand "Roll the omelette into a French fold with no browning at all" --side-of "$serve"
for q in "$salt" "$pan" "$butter"; do quiet "$m" fulfil "$q"; done
quiet "$m" take-up "$shop" --by erkan

# J4: brunch waits on the whole omelette journey (drawn as one journey card).
quiet "$m" journey new "Host brunch for four on Sunday" 2>/dev/null
brunch=$("$m" errand "Sit four people down to brunch at eleven" --crowns J4 --json | key)
quiet "$m" require "$brunch" J3
table=$("$m" errand "Set the table for four with the good plates" --opens "$brunch" --json | key)
quiet "$m" errand "Brew a pot of Turkish tea" --opens "$brunch"
quiet "$m" petition "Confirm who is coming" --on "the group chat" --opens "$brunch"
quiet "$m" errand "Bake simit the night before" --side-of "$brunch"
quiet "$m" fulfil "$table"

# J5: a code journey. Its GitHub issue needs gh; without it, the journey is
# errands only.
quiet "$m" journey new "mikado can hook in any task tool, not only GitHub" 2>/dev/null
release=$("$m" errand "Ship pluggable quest sources in a release" --crowns J5 --json | key)
before=$release
if issue=$("$m" add erkantaylan/mikado#3 --opens "$release" --json 2>/dev/null | key); then
	before=$issue
else
	echo "seed.sh: could not add erkantaylan/mikado#3 (is gh logged in?); J5 has no issue quest" >&2
fi
protocol=$("$m" errand "Write down the JSON protocol a source speaks" --opens "$before" --json | key)
github=$("$m" errand "Move the GitHub code behind the source interface" --opens "$release" --json | key)
quiet "$m" require "$github" "$protocol"
quiet "$m" errand "Write a beads source as the first outside example" --side-of "$release"
quiet "$m" take-up "$protocol" --by claude

"$m" journey list --all
