#!/usr/bin/env bash
# Fills an empty mikado server with the demo journeys used for development,
# testing and the README screenshots: goals with and without code.
#
#   MIKADO_SERVER=http://127.0.0.1:47295 src/demo/seed.sh [path/to/mikado]
#
# Run it against a fresh data directory (make demo does): the keys it makes
# (J1…J7, Q1…) depend on starting from nothing.
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
a=$("$m" add "Carry the desk and chair into the spare room" --crowns J1 --hero defne --json | key)
b=$("$m" add "Clear the old boxes out of the spare room" --opens "$a" --hero defne --json | key)
c=$("$m" add "Run a network cable along the skirting board" --opens "$a" --hero defne --json | key)
s=$("$m" add "Hang the map of Istanbul above the desk" --side-of "$a" --json | key)
for q in "$b" "$c" "$a" "$s"; do quiet "$m" fulfil "$q"; done

# J2: main quest fulfilled, one achievement left.
quiet "$m" journey new "Touch-type at 60 words a minute" 2>/dev/null
a=$("$m" add "Pass a 60 wpm test three days in a row" --crowns J2 --json | key)
b=$("$m" add "Practise 15 minutes every morning for a month" --opens "$a" --json | key)
quiet "$m" add "Reach 80 wpm on a code-typing test" --side-of "$a"
s=$("$m" add "Learn the symbols row without looking" --side-of "$a" --json | key)
for q in "$b" "$a" "$s"; do quiet "$m" fulfil "$q"; done

# J3: an omelette — sealed, open, underway, fulfilled and abandoned quests,
# a found quest, side quests on side quests, a petition, and a crowning quest
# linked to its recipe with a mark.
quiet "$m" journey new "Make an omelette" 2>/dev/null
serve=$("$m" add "Serve the omelette on a warm plate while it is still soft in the middle" --crowns J3 \
	--url https://recipes.example.com/french-omelette --mark recipe --json | key)
whisk=$("$m" add "Whisk three eggs with a pinch of salt until no streaks of white remain" --opens "$serve" --json | key)
heat=$("$m" add "Heat a knob of butter in the pan until it foams but doesn't brown" --opens "$serve" --json | key)
eggs=$("$m" add "Make sure there are three eggs in the fridge" --opens "$whisk" --hero defne --json | key)
salt=$("$m" add "Make sure there is salt in the grinder or the jar" --opens "$whisk" --json | key)
pan=$("$m" add "Wash and dry the 20 cm non-stick pan" --opens "$heat" --hero defne --json | key)
butter=$("$m" add "Take the butter out of the fridge and cut a 15 g knob" --opens "$heat" --json | key)
shop=$("$m" add "Buy a box of eggs from the corner shop" --opens "$eggs" --found-on "$eggs" \
	--reason "only two eggs left, the recipe needs three" --hero defne --json | key)
filling=$("$m" add "Grate a handful of cheese and chop the chives for a filling" --side-of "$serve" --json | key)
quiet "$m" add "Ask your flatmate whether the cheddar in the fridge is theirs" --on flatmate --side-of "$filling"
quiet "$m" add "Grind black pepper into the eggs just before cooking" --side-of "$whisk"
quiet "$m" add "Roll the omelette into a French fold with no browning at all" --side-of "$serve"
for q in "$salt" "$pan" "$butter"; do quiet "$m" fulfil "$q"; done
quiet "$m" take-up "$shop" --by defne
castiron=$("$m" add "Borrow the flatmate's cast-iron pan" --opens "$heat" --json | key)
quiet "$m" abandon "$castiron" --reason "the non-stick pan does the job"

# J4: brunch waits on the whole omelette journey (drawn as one journey card).
quiet "$m" journey new "Host brunch for four on Sunday" 2>/dev/null
brunch=$("$m" add "Sit four people down to brunch at eleven" --crowns J4 --json | key)
quiet "$m" require "$brunch" J3
table=$("$m" add "Set the table for four with the good plates" --opens "$brunch" --json | key)
tea=$("$m" add "Pour everyone a glass of Turkish tea" --opens "$brunch" --json | key)
steep=$("$m" add "Let the tea steep on the çaydanlık for fifteen minutes" --opens "$tea" --json | key)
water=$("$m" add "Bring the water in the lower kettle to the boil" --opens "$steep" --json | key)
leaves=$("$m" add "Put four spoons of black tea in the upper pot" --opens "$steep" \
	--url https://recipes.example.com/turkish-tea --json | key)
quiet "$m" add "Confirm who is coming" --on "the group chat" --opens "$brunch"
simit=$("$m" add "Bake simit the night before" --side-of "$brunch" --json | key)
quiet "$m" abandon "$simit" --reason "the bakery on the corner sells them fresh"
quiet "$m" fulfil "$table"

# J5: a code journey on a made-up recipe site: quests marked with its issue
# numbers and a commit hash, each linked to its page.
quiet "$m" journey new "The recipe site gets a dark mode" 2>/dev/null
git=https://git.example.com/recipe-site
release=$("$m" add "Ship dark mode in a release" --crowns J5 --json | key)
toggle=$("$m" add "Add a light / dark switch to the header" --opens "$release" \
	--url "$git/issues/12" --mark "#12" --json | key)
palette=$("$m" add "Pick a dark palette that keeps the food photos true to colour" --opens "$release" \
	--url "$git/issues/9" --mark "#9" --json | key)
vars=$("$m" add "Move every colour into CSS variables" --opens "$toggle" --opens "$palette" \
	--url "$git/commit/4e1f9a7" --mark 4e1f9a7 --json | key)
quiet "$m" add "Ask the photographer for darker backgrounds" --on "the photographer" --opens "$palette" --mark "#14"
quiet "$m" add "Remember the choice across visits" --side-of "$toggle" --mark "feat(prefs)"
quiet "$m" fulfil "$vars"
quiet "$m" take-up "$toggle" --by mira

# J6: extracted from J4 — the tea grew into a journey of its own, drawn on
# J4's war table as one journey card.
quiet "$m" journey extract J4 "$steep" "$water" "$leaves" --title "Brew Turkish tea in the çaydanlık"
quiet "$m" fulfil "$water"

# J7: tea on the balcony waits on the whole tea journey too, so the tea's
# crowning quest is in three journeys (J6, J4 and this one).
quiet "$m" journey new "Have the neighbours over for tea on the balcony" 2>/dev/null
balcony=$("$m" add "Pour the neighbours tea with a plate of baklava on the balcony" --crowns J7 --json | key)
quiet "$m" require "$balcony" J6
quiet "$m" add "Buy a tray of pistachio baklava from the bakery" --opens "$balcony"

# Regions: the kitchen journeys wait on each other, so they move together; the
# rest stay in R1, Personal.
quiet "$m" region new "Kitchen"
quiet "$m" region move Kitchen J3 J4 J6 J7
quiet "$m" region new "Side projects"
quiet "$m" region move "Side projects" J5

"$m" journey list --all
