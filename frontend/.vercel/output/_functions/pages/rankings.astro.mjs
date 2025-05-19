import { c as createComponent, e as renderComponent, f as renderScript, r as renderTemplate, m as maybeRenderHead, b as addAttribute } from '../chunks/astro/server_C_1jQ3jI.mjs';
import 'kleur/colors';
import { $ as $$Layout } from '../chunks/Layout_BZw4Q8am.mjs';
import { s as supabase } from '../chunks/supabase_CyFqHHS4.mjs';
export { renderers } from '../renderers.mjs';

const $$Index = createComponent(async ($$result, $$props, $$slots) => {
  const weightClasses = [
    "Flyweight",
    "Bantamweight",
    "Featherweight",
    "Lightweight",
    "Welterweight",
    "Middleweight",
    "Light Heavyweight",
    "Heavyweight",
    "Women's Strawweight",
    "Women's Flyweight",
    "Women's Bantamweight",
    "Women's Featherweight"
  ];
  const { data: rankings, error: rankingsError } = await supabase.from("fighter_rankings").select(`
    id,
    fighter_id,
    weight_class,
    rank,
    fighter:fighter_id (
      id,
      name,
      nickname,
      wins,
      losses,
      draws,
      status
    )
  `);
  if (rankingsError) {
    console.error("Error fetching rankings:", rankingsError);
  }
  const { data: rankedFighters, error: fightersError } = await supabase.from("fighters").select("*").not("rank", "is", null).not("rank", "eq", "Unranked").not("rank", "eq", "NR");
  if (fightersError) {
    console.error("Error fetching fighters:", fightersError);
  }
  function getRankValue(rank) {
    if (!rank) return 999;
    if (rank === "Champion") return -2;
    if (rank === "Interim Champion") return -1;
    if (rank.startsWith("#")) {
      const rankNum2 = parseInt(rank.substring(1));
      return isNaN(rankNum2) ? 999 : rankNum2;
    }
    const rankNum = parseInt(rank);
    return !isNaN(rankNum) ? rankNum : 999;
  }
  const fightersByWeightClass = {};
  weightClasses.forEach((weightClass) => {
    fightersByWeightClass[weightClass] = [];
  });
  if (rankings) {
    rankings.forEach((ranking) => {
      const weightClass = ranking.weight_class;
      const rank = ranking.rank;
      const fighter = ranking.fighter;
      if (rank === "Champion" || rank === "Interim Champion" || rank.startsWith("#") || !isNaN(parseInt(rank))) {
        if (weightClasses.includes(weightClass) && fighter) {
          fightersByWeightClass[weightClass].push({
            id: fighter.id,
            name: fighter.name,
            nickname: fighter.nickname,
            wins: fighter.wins || 0,
            losses: fighter.losses || 0,
            draws: fighter.draws || 0,
            status: fighter.status,
            currentRank: rank,
            currentWeightClass: weightClass
          });
        }
      }
    });
  }
  if (rankedFighters) {
    rankedFighters.forEach((fighter) => {
      const hasRankingEntry = rankings?.some(
        (ranking) => ranking.fighter_id === fighter.id
      );
      if (!hasRankingEntry && fighter.rank && fighter.weight_class) {
        const weightClass = fighter.weight_class;
        if (weightClasses.includes(weightClass)) {
          fightersByWeightClass[weightClass].push({
            id: fighter.id,
            name: fighter.name,
            nickname: fighter.nickname,
            wins: fighter.wins || 0,
            losses: fighter.losses || 0,
            draws: fighter.draws || 0,
            status: fighter.status,
            currentRank: fighter.rank,
            currentWeightClass: weightClass
          });
        }
      }
    });
  }
  for (const weightClass in fightersByWeightClass) {
    fightersByWeightClass[weightClass].sort((a, b) => {
      return getRankValue(a.currentRank) - getRankValue(b.currentRank);
    });
  }
  const activeWeightClasses = weightClasses.filter(
    (wc) => fightersByWeightClass[wc] && fightersByWeightClass[wc].length > 0
  );
  const selectedWeightClass = activeWeightClasses.length > 0 ? activeWeightClasses[0] : "Heavyweight";
  return renderTemplate`${renderComponent($$result, "Layout", $$Layout, { "title": "UFC Rankings" }, { "default": async ($$result2) => renderTemplate` ${maybeRenderHead()}<main class="rankings-page"> <h1 class="rankings-title">UFC Rankings</h1> <div class="rankings-nav-links"> <a href="/fighters" class="rankings-nav-link">All Fighters</a> <a href="/rankings" class="rankings-nav-link active">Rankings</a> </div> <div class="rankings-weight-class-tabs"> ${activeWeightClasses.map((weightClass) => renderTemplate`<button${addAttribute(`rankings-weight-tab ${weightClass === selectedWeightClass ? "active" : ""}`, "class")}${addAttribute(weightClass, "data-weight-class")}> ${weightClass} </button>`)} </div> ${activeWeightClasses.map((weightClass) => renderTemplate`<div${addAttribute(`rankings-weight-class-section ${weightClass === selectedWeightClass ? "active" : ""}`, "class")}${addAttribute(`weight-class-${weightClass.replace(/\s+/g, "-").replace(/'/g, "").toLowerCase()}`, "id")}> <div class="rankings-fighters-grid"> ${fightersByWeightClass[weightClass].map((fighter) => renderTemplate`<a${addAttribute(`/fighters/${fighter.id}`, "href")}${addAttribute(`rankings-fighter-card ${fighter.currentRank === "Champion" ? "rankings-champion" : fighter.currentRank === "Interim Champion" ? "rankings-interim-champion" : ""}`, "class")}> <div class="rankings-fighter-info"> <h3 class="rankings-fighter-name">${fighter.name}</h3> ${fighter.nickname && renderTemplate`<p class="rankings-fighter-nickname">${fighter.nickname}</p>`} <div class="rankings-fighter-record"> <span class="rankings-record-numbers">${fighter.wins}-${fighter.losses}-${fighter.draws}</span> <span class="rankings-record-label">Record</span> </div> </div> <div class="rankings-fighter-rank">${fighter.currentRank}</div> </a>`)} </div> </div>`)} </main> ` })} ${renderScript($$result, "C:/Users/richa/OneDrive/Desktop/MMA-Scheduler/frontend/src/pages/rankings/index.astro?astro&type=script&index=0&lang.ts")}`;
}, "C:/Users/richa/OneDrive/Desktop/MMA-Scheduler/frontend/src/pages/rankings/index.astro", void 0);

const $$file = "C:/Users/richa/OneDrive/Desktop/MMA-Scheduler/frontend/src/pages/rankings/index.astro";
const $$url = "/rankings";

const _page = /*#__PURE__*/Object.freeze(/*#__PURE__*/Object.defineProperty({
  __proto__: null,
  default: $$Index,
  file: $$file,
  url: $$url
}, Symbol.toStringTag, { value: 'Module' }));

const page = () => _page;

export { page };
