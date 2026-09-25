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
    term: 'Quest',
    text: 'A goal: small enough to finish, big enough to need several deeds. It is drawn as a chart that ends in one crowning deed.',
    cli: 'mikado quest new "title"',
  },
  {
    id: 'G2',
    term: 'Quest Board',
    text: 'The page with every quest at a glance: progress on its main quest, its achievements, and who is on it.',
    cli: 'mikado quest list · mikado open',
  },
  {
    id: 'G3',
    term: 'Chart',
    text: 'One quest drawn out: every deed on the way to the crowning deed, and what requires what.',
    cli: 'mikado quest show SLUG · mikado open SLUG',
  },
  {
    id: 'G4',
    term: 'Deed',
    also: 'Task',
    text: 'Any unit on the chart, named by its id, like M142. "Task" means exactly the same; either word works.',
    cli: 'mikado show D',
  },
  {
    id: 'G5',
    term: 'Issue',
    text: 'A deed that is a GitHub issue (owner/repo#n). Its title, state and assignees come from GitHub; it is fulfilled when the issue is closed.',
    cli: 'mikado add owner/repo#n',
  },
  {
    id: 'G6',
    term: 'Errand',
    text: 'A deed that is a real step but not worth a GitHub issue, like booking a release window.',
    cli: 'mikado errand "title"',
  },
  {
    id: 'G7',
    term: 'Petition',
    text: 'A deed that waits on someone, often outside the team, like a production token from a vendor. It says whom it awaits a reply from, and since when.',
    cli: 'mikado petition "title" --on WHO',
  },
  {
    id: 'G8',
    term: 'Crowning deed',
    text: 'The one deed whose fulfilment fulfils the quest. Everything else on the main quest leads to it.',
    cli: 'mikado quest crown SLUG D · --crowns SLUG',
  },
  {
    id: 'G9',
    term: 'Main quest',
    text: 'The required path: the crowning deed and every deed it requires, directly or not. Its progress is the quest’s progress.',
  },
  {
    id: 'G10',
    term: 'Side quest',
    text: 'Optional polish hung on a deed. It never blocks anything and counts as an achievement, not as progress.',
    cli: '--side-of D',
  },
  {
    id: 'G11',
    term: 'Achievement',
    text: 'What a fulfilled side quest earns: a star beside the quest’s progress, never a requirement.',
  },
  {
    id: 'G12',
    term: 'Requires / Opens',
    text: 'The only blocking link. "M5 requires M3" means M3 must be fulfilled first; said the other way round, M3 opens M5. Cycles are refused.',
    cli: 'mikado require D PREREQ · --requires D · --opens D',
  },
  {
    id: 'G13',
    term: 'Unearthed',
    text: 'A deed found while working on another, recorded with where and why: "unearthed while on M3: old saves crash the loader".',
    cli: '--unearthed-on D --reason "why"',
  },
  {
    id: 'G14',
    term: 'Fulfilled',
    text: 'Status: finished. An issue is fulfilled when it closes on GitHub; an errand or a petition when it is marked so.',
    cli: 'mikado fulfil D',
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
    cli: 'mikado abandon D --reason "why" · mikado unabandon D',
  },
  {
    id: 'G19',
    term: 'Struck',
    text: 'Gone for good: taken off the chart with its side quests, and kept only in the chronicle. Prefer abandoning.',
    cli: 'mikado strike D --reason "why"',
  },
  {
    id: 'G20',
    term: 'Underway',
    text: 'Someone is on it right now, apart from its status. You take a deed up when you start and set it down when you stop.',
    cli: 'mikado take-up D [--by WHO] · mikado set-down D',
  },
  {
    id: 'G21',
    term: 'Hero / no hero',
    text: 'Whoever is responsible for a deed: its GitHub assignee, else the hero set in mikado. A deed with neither shows "no hero".',
    cli: '--hero WHO · mikado set D --hero WHO',
  },
  {
    id: 'G22',
    term: 'Chronicle',
    text: 'The history: every change as one sentence. A quest’s chronicle is its own events plus those of the deeds on its chart.',
    cli: 'mikado quest show SLUG',
  },
  {
    id: 'G23',
    term: 'NPC',
    text: 'A decorative joke badge any deed can wear. Nothing reads it.',
    cli: '--npc',
  },
  {
    id: 'G24',
    term: 'Archived',
    text: 'A quest put away: off the Quest Board’s shelves and out of quest list, its chart, deeds and chronicle untouched. Bring it back any time.',
    cli: 'mikado quest archive SLUG · mikado quest unarchive SLUG',
  },
]
