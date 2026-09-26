// The words mikado uses, one place for the dashboard. The Glossary tab draws this list;
// SKILL.md carries the same table for agents (src/internal/skill/SKILL.md), so change both together.

export type Term = {
  id: string // G1…G24, as SKILL.md numbers them
  term: string
  also?: string // a synonym that means exactly the same
  text: string // what it means, in a sentence or two
  cli?: string // the command or flag that does it, if there is one
}

export const glossary: Term[] = [
  {
    id: 'G1',
    term: 'Journey',
    text: 'A goal: small enough to finish, big enough to need several quests. It is named by its key, like J7, and drawn as a chart that ends in one crowning quest.',
    cli: 'mikado journey new "title"',
  },
  {
    id: 'G2',
    term: 'Atlas',
    text: 'The page with every journey at a glance: progress on its main quest, its achievements, and who is on it.',
    cli: 'mikado journey list · mikado open',
  },
  {
    id: 'G3',
    term: 'Chart',
    text: 'One journey drawn out: every quest on the way to the crowning quest, and what requires what. Another journey this one waits on is drawn as one card, with its progress and a link to its own chart.',
    cli: 'mikado journey show J · mikado open J',
  },
  {
    id: 'G4',
    term: 'Quest',
    also: 'Task',
    text: 'Any unit on the chart, named by its key, like Q142. "Task" means exactly the same; either word works.',
    cli: 'mikado show Q',
  },
  {
    id: 'G5',
    term: 'Issue',
    text: 'A quest that is a GitHub issue (owner/repo#n). Its title, state and assignees come from GitHub; it is fulfilled when the issue is closed.',
    cli: 'mikado add owner/repo#n',
  },
  {
    id: 'G6',
    term: 'Errand',
    text: 'A quest that is a real step but not worth a GitHub issue, like booking a release window.',
    cli: 'mikado errand "title"',
  },
  {
    id: 'G7',
    term: 'Petition',
    text: 'A quest that waits on someone, often outside the team, like a production token from a vendor. It says whom it awaits a reply from, and since when.',
    cli: 'mikado petition "title" --on WHO',
  },
  {
    id: 'G8',
    term: 'Crowning quest',
    text: 'The one quest whose fulfilment fulfils the journey. Everything else on the main quest leads to it.',
    cli: 'mikado journey crown J Q · --crowns J',
  },
  {
    id: 'G9',
    term: 'Main quest',
    text: 'The required path: the crowning quest and every quest it requires, directly or not. Its progress is the journey’s progress.',
  },
  {
    id: 'G10',
    term: 'Side quest',
    text: 'Optional polish hung on a quest. It never blocks anything and counts as an achievement, not as progress.',
    cli: '--side-of Q',
  },
  {
    id: 'G11',
    term: 'Achievement',
    text: 'What a fulfilled side quest earns: a star beside the journey’s progress, never a requirement.',
  },
  {
    id: 'G12',
    term: 'Requires / Opens',
    text: 'The only blocking link. "Q5 requires Q3" means Q3 must be fulfilled first; said the other way round, Q3 opens Q5. Cycles are refused.',
    cli: 'mikado require Q PREREQ · --requires Q · --opens Q',
  },
  {
    id: 'G13',
    term: 'Found',
    text: 'A quest found while working on another, recorded with where and why: "found on Q3: old saves crash the loader".',
    cli: '--found-on Q --reason "why"',
  },
  {
    id: 'G14',
    term: 'Fulfilled',
    text: 'Status: finished. An issue is fulfilled when it closes on GitHub; an errand or a petition when it is marked so.',
    cli: 'mikado fulfil Q',
  },
  {
    id: 'G15',
    term: 'Open',
    text: 'Status: everything it requires is fulfilled (or abandoned), so it can be started now.',
  },
  {
    id: 'G16',
    term: 'Sealed',
    text: 'Status: blocked. Something it requires is not fulfilled yet; the badge counts what is left, as in "sealed · 2 to go".',
  },
  {
    id: 'G17',
    term: 'Awaiting reply',
    text: 'Status of a petition that is otherwise open: only the person it waits on stands in the way.',
  },
  {
    id: 'G18',
    term: 'Abandoned',
    text: 'Status: won’t be done. It stays on the chart, blocks nothing and counts in no total; its side quests are abandoned with it. An issue closed on GitHub as not planned or duplicate reads as abandoned.',
    cli: 'mikado abandon Q --reason "why" · mikado unabandon Q',
  },
  {
    id: 'G19',
    term: 'Struck',
    text: 'Gone for good: taken off the chart with its side quests, and kept only in the chronicle. Prefer abandoning.',
    cli: 'mikado strike Q --reason "why"',
  },
  {
    id: 'G20',
    term: 'Underway',
    text: 'Someone is on it right now, apart from its status. You take a quest up when you start and set it down when you stop.',
    cli: 'mikado take-up Q [--by WHO] · mikado set-down Q',
  },
  {
    id: 'G21',
    term: 'Hero / no hero',
    text: 'Whoever is responsible for a quest: its GitHub assignee, else the hero set in mikado. A quest with neither shows "no hero".',
    cli: '--hero WHO · mikado set Q --hero WHO',
  },
  {
    id: 'G22',
    term: 'Chronicle',
    text: 'The history: every change as one sentence. A journey’s chronicle is its own events plus those of the quests on its chart.',
    cli: 'mikado journey show J',
  },
  {
    id: 'G23',
    term: 'NPC',
    text: 'A mark any quest can wear. It changes nothing about the quest’s status, but NPC quests show in red on the chart; right-click a quest there to mark or unmark it.',
    cli: '--npc · mikado set Q --npc=true|false',
  },
  {
    id: 'G24',
    term: 'Archived',
    text: 'A journey put away: off the Atlas’s shelves and out of journey list, its chart, quests and chronicle untouched. Bring it back any time.',
    cli: 'mikado journey archive J · mikado journey unarchive J',
  },
]
