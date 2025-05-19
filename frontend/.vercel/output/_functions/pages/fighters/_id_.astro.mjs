import { c as createComponent, a as createAstro, e as renderComponent, r as renderTemplate, m as maybeRenderHead, f as renderScript, F as Fragment, b as addAttribute } from '../../chunks/astro/server_C_1jQ3jI.mjs';
import 'kleur/colors';
import { $ as $$Layout } from '../../chunks/Layout_BZw4Q8am.mjs';
import { s as supabase } from '../../chunks/supabase_CyFqHHS4.mjs';
export { renderers } from '../../renderers.mjs';

const $$Astro = createAstro();
const $$id = createComponent(async ($$result, $$props, $$slots) => {
  const Astro2 = $$result.createAstro($$Astro, $$props, $$slots);
  Astro2.self = $$id;
  const { id } = Astro2.params;
  const fighterId = id;
  if (!fighterId) {
    return Astro2.redirect("/fighters");
  }
  const { data: fighter, error: fighterError } = await supabase.from("fighters").select("*").eq("id", fighterId).single();
  if (fighterError) {
    console.error("Error fetching fighter:", fighterError);
  }
  const fighterData = fighter;
  const { data: rankings, error: rankingsError } = await supabase.from("fighter_rankings").select("*").eq("fighter_id", fighterId);
  if (rankingsError) {
    console.error("Error fetching fighter rankings:", rankingsError);
  }
  const fighterRankings = rankings || [];
  const { data: fights1, error: fights1Error } = await supabase.from("fights").select(
    `
    *,
    event:event_id(id, name, event_date)
  `
  ).eq("fighter1_id", fighterId);
  const { data: fights2, error: fights2Error } = await supabase.from("fights").select(
    `
    *,
    event:event_id(id, name, event_date)
  `
  ).eq("fighter2_id", fighterId);
  if (fights1Error) {
    console.error("Error fetching fights as fighter1:", fights1Error);
  }
  if (fights2Error) {
    console.error("Error fetching fights as fighter2:", fights2Error);
  }
  const allFights = [...fights1 || [], ...fights2 || []];
  const sortedFights = allFights.sort((a, b) => {
    const dateA = new Date(a.event.event_date);
    const dateB = new Date(b.event.event_date);
    return dateB.getTime() - dateA.getTime();
  });
  const isFightCanceled = (fight) => {
    const eventDate = new Date(fight.event.event_date);
    const now = /* @__PURE__ */ new Date();
    const isEventCompleted = eventDate < now;
    return isEventCompleted && !fight.winner_id && (!fight.result_method || fight.result_method !== "Draw" && fight.result_method !== "No Contest");
  };
  return renderTemplate`${renderComponent($$result, "Layout", $$Layout, { "title": fighterData ? fighterData.name : "Fighter Details" }, { "default": async ($$result2) => renderTemplate` ${maybeRenderHead()}<main class="fighter-page"> ${fighterData ? renderTemplate`<div class="fighter-container"> <div class="fighter-header"> <div class="fighter-info"> <h1>${fighterData.name}</h1>  ${fighterData.nickname && renderTemplate`<p class="fighter-nickname">${fighterData.nickname}</p>`}  ${fighterRankings.length > 0 ? renderTemplate`<div class="fighter-rankings"> ${fighterRankings.map((ranking) => renderTemplate`<div class="fighter-rank"> ${ranking.rank === "Champion" ? renderTemplate`<span class="champion-badge">Champion</span>` : ranking.rank === "Unranked" ? renderTemplate`${renderComponent($$result2, "Fragment", Fragment, {}, { "default": async ($$result3) => renderTemplate`Unranked` })}` : renderTemplate`${renderComponent($$result2, "Fragment", Fragment, {}, { "default": async ($$result3) => renderTemplate`Ranked ${ranking.rank}` })}`} <span class="rank-division"> ${" "}
at ${ranking.weight_class} </span> </div>`)} </div>` : (
    /* Fall back to single weight class and rank if no multiple rankings */
    renderTemplate`${renderComponent($$result2, "Fragment", Fragment, {}, { "default": async ($$result3) => renderTemplate`${fighterData.weight_class && renderTemplate`<p class="fighter-weight-class"> ${fighterData.weight_class} </p>`}${fighterData.rank && fighterData.rank !== "NR" && renderTemplate`<div class="fighter-rank"> ${fighterData.rank === "Champion" ? renderTemplate`<span class="champion-badge">Champion</span>` : fighterData.rank === "Unranked" ? renderTemplate`${renderComponent($$result3, "Fragment", Fragment, {}, { "default": async ($$result4) => renderTemplate`Unranked` })}` : renderTemplate`${renderComponent($$result3, "Fragment", Fragment, {}, { "default": async ($$result4) => renderTemplate`Ranked ${fighterData.rank}` })}`} </div>`}` })}`
  )} </div> ${fighterData.ufc_url && renderTemplate`<a${addAttribute(fighterData.ufc_url, "href")} target="_blank" rel="noopener noreferrer" class="ufc-profile-link">
UFC Profile
</a>`} </div> <div class="fighter-profile"> <div class="fighter-stats"> <h2>Fighter Stats</h2> <div class="stats-grid"> <div class="stat-item"> <span class="stat-label">Status</span> <span class="stat-value"> ${fighterData.status || "Unknown"} </span> </div> ${fighterData.age !== null && fighterData.age !== void 0 && fighterData.age > 0 && renderTemplate`<div class="stat-item"> <span class="stat-label">Age</span> <span class="stat-value">${fighterData.age}</span> </div>`} ${fighterData.height && renderTemplate`<div class="stat-item"> <span class="stat-label">Height</span> <span class="stat-value">${fighterData.height}</span> </div>`} ${fighterData.weight && renderTemplate`<div class="stat-item"> <span class="stat-label">Weight</span> <span class="stat-value">${fighterData.weight}</span> </div>`} ${fighterData.reach && renderTemplate`<div class="stat-item"> <span class="stat-label">Reach</span> <span class="stat-value">${fighterData.reach}</span> </div>`} ${fighterData.nationality && renderTemplate`<div class="stat-item"> <span class="stat-label">Nationality</span> <span class="stat-value">${fighterData.nationality}</span> </div>`} ${fighterData.fighting_out_of && renderTemplate`<div class="stat-item fighting-locations"> <span class="stat-label">Fighting Out Of</span> ${fighterData.fighting_out_of.includes("{") ? (() => {
    const locations = fighterData.fighting_out_of.replace(/^\{|\}$/g, "").split("}, {");
    if (locations.length === 1) {
      return renderTemplate`<span class="stat-value">${locations[0]}</span>`;
    }
    return renderTemplate`<ul class="locations-list"> ${locations.map((location) => renderTemplate`<li>${location}</li>`)} </ul>`;
  })() : renderTemplate`<span class="stat-value"> ${fighterData.fighting_out_of} </span>`} </div>`} </div> </div> <div class="fighter-record"> <h2>Fight Record</h2> <div class="record-card"> <div class="record-main"> <div class="win-box"> <span class="count">${fighterData.wins || 0}</span> <span class="label">WINS</span> </div> <div class="loss-box"> <span class="count">${fighterData.losses || 0}</span> <span class="label">LOSSES</span> </div> <div class="draw-box"> <span class="count">${fighterData.draws || 0}</span> <span class="label">DRAWS</span> </div> ${fighterData.no_contests && fighterData.no_contests > 0 ? renderTemplate`<div class="nc-box"> <span class="count">${fighterData.no_contests}</span> <span class="label">NC</span> </div>` : null} </div> </div> <div class="win-methods"> ${fighterData.ko_wins !== null && fighterData.sub_wins !== null && fighterData.dec_wins !== null && renderTemplate`<div class="methods-container"> <h3>Win Methods</h3> <div class="method-items"> <div class="method-item"> <span class="method-value"> ${fighterData.ko_wins} </span> <span class="method-label">KO/TKO</span> </div> <div class="method-item"> <span class="method-value"> ${fighterData.sub_wins} </span> <span class="method-label">Submission</span> </div> <div class="method-item"> <span class="method-value"> ${fighterData.dec_wins} </span> <span class="method-label">Decision</span> </div> </div> </div>`} ${fighterData.loss_by_ko !== null && fighterData.loss_by_sub !== null && fighterData.loss_by_dec !== null && renderTemplate`<div class="methods-container"> <h3>Loss Methods</h3> <div class="method-items"> <div class="method-item"> <span class="method-value"> ${fighterData.loss_by_ko} </span> <span class="method-label">KO/TKO</span> </div> <div class="method-item"> <span class="method-value"> ${fighterData.loss_by_sub} </span> <span class="method-label">Submission</span> </div> <div class="method-item"> <span class="method-value"> ${fighterData.loss_by_dec} </span> <span class="method-label">Decision</span> </div>  ${fighterData.loss_by_dq !== null && fighterData.loss_by_dq !== void 0 && fighterData.loss_by_dq > 0 && renderTemplate`<div class="method-item"> <span class="method-value"> ${fighterData.loss_by_dq} </span> <span class="method-label">DQ</span> </div>`} </div> </div>`} </div> </div> </div> <div class="fighter-fights"> <h2>Fight History</h2> ${sortedFights.length > 0 ? renderTemplate`<div> <div class="fights-list" id="fights-list"> ${sortedFights.slice(0, 3).map((fight) => {
    const isMainEvent = fight.is_main_event;
    const isTitleFight = fight.was_title_fight;
    const fighterName = fighter.name;
    const opponent = fight.fighter1_id === fighterId ? fight.fighter2_name : fight.fighter1_name;
    const opponentId = fight.fighter1_id === fighterId ? fight.fighter2_id : fight.fighter1_id;
    let fighterRank = fight.fighter1_id === fighterId ? fight.fighter1_rank : fight.fighter2_rank;
    let opponentRank = fight.fighter1_id === fighterId ? fight.fighter2_rank : fight.fighter1_rank;
    if (fighterRank === "#C" || fighterRank === "C") {
      fighterRank = "Champion";
    }
    if (opponentRank === "#C" || opponentRank === "C") {
      opponentRank = "Champion";
    }
    const eventDate = new Date(fight.event.event_date);
    const isCanceled = isFightCanceled(fight);
    const isWin = fight.winner_id === fighterId;
    let result = "";
    if (isCanceled) {
      result = "CANCELED";
    } else if (fight.winner_id) {
      result = isWin ? "WIN" : "LOSS";
    } else if (fight.result_method === "Draw") {
      result = "DRAW";
    } else if (fight.result_method === "No Contest") {
      result = "NC";
    }
    return renderTemplate`<div${addAttribute(`fight-card ${isMainEvent ? "main-event" : ""} ${isTitleFight ? "title-fight" : ""} result-${result.toLowerCase()}`, "class")}> <div class="fight-date"> <span> ${eventDate.toLocaleDateString("en-US", {
      year: "numeric",
      month: "short",
      day: "numeric"
    })} </span> <div class="event-badges"> <span class="weight-class-badge"> ${fight.weight_class} </span> ${isMainEvent && renderTemplate`<span class="main-event-badge">Main Event</span>`} ${isTitleFight && renderTemplate`<span class="title-fight-badge">Title Fight</span>`} ${isCanceled && renderTemplate`<span class="canceled-badge">Canceled</span>`} </div> </div> <div class="fight-details"> <a${addAttribute(`/events/event/${fight.event_id}`, "href")} class="event-link"> <span class="event-name">${fight.event.name}</span> </a> <div class="matchup"> <div class="fighter-vs"> <div${addAttribute(`fighter-name-container${!isCanceled && isWin ? " winner" : ""} ${isCanceled ? "canceled" : ""}`, "class")}> <a${addAttribute(`/fighters/${fighterId}`, "href")} class="fighter-link"> ${fighterName} </a> ${fighterRank && renderTemplate`<span${addAttribute(`rank ${fighterRank === "Champion" ? "champion" : ""}`, "class")}> ${fighterRank === "Champion" ? "C" : !fighterRank?.trim() ? "Unranked" : `#${fighterRank}`} </span>`} </div> <span class="vs">vs</span> <div${addAttribute(`fighter-name-container${!isCanceled && !isWin && fight.winner_id ? " winner" : ""} ${isCanceled ? "canceled" : ""}`, "class")}> <a${addAttribute(`/fighters/${opponentId}`, "href")} class="fighter-link"> ${opponent} </a> ${opponentRank !== null && opponentRank !== void 0 && renderTemplate`<span${addAttribute(`rank ${opponentRank === "Champion" ? "champion" : ""}`, "class")}> ${opponentRank === "Champion" ? "C" : opponentRank === "" ? "Unranked" : `#${opponentRank}`} </span>`} </div> <span${addAttribute(`result-badge ${result.toLowerCase()}`, "class")}> ${result} </span> </div> </div> ${isCanceled ? renderTemplate`<div class="result-method canceled">
Fight Canceled
</div>` : fight.result_method ? renderTemplate`<div class="result-method"> ${fight.result_method} ${fight.result_method_details && ` (${fight.result_method_details})`} ${fight.result_round && ` R${fight.result_round}`} ${fight.result_time && ` ${// Check if the time is duplicated (e.g., "00:04:1400:04:14")
    fight.result_time.length % 2 === 0 && fight.result_time.substring(
      0,
      fight.result_time.length / 2
    ) === fight.result_time.substring(
      fight.result_time.length / 2
    ) ? fight.result_time.substring(
      0,
      fight.result_time.length / 2
    ) : fight.result_time}`} </div>` : null} </div> </div>`;
  })} </div> ${sortedFights.length > 3 && renderTemplate`<div class="load-more-container"> <button id="load-more-fights" class="load-more-button">
Load More
</button> <p class="fights-count">
Showing <span id="shown-fights-count">3</span> of${" "} <span id="total-fights-count">${sortedFights.length}</span>${" "}
fights
</p> </div>`} <div id="all-fights-data" style="display: none;"> ${JSON.stringify(sortedFights)} </div> <div id="fighter-id-data" style="display: none;"> ${fighterId} </div> </div>` : renderTemplate`<div class="no-fights"> <p>No fights on record.</p> </div>`} </div> </div>` : renderTemplate`<div class="fighter-not-found"> <h1>Fighter Not Found</h1> <p>
Sorry, the fighter you're looking for doesn't exist or has been
            removed.
</p> <a href="/fighters" class="back-button">
Back to Fighters
</a> </div>`} ${renderScript($$result2, "C:/Users/richa/OneDrive/Desktop/MMA-Scheduler/frontend/src/pages/fighters/[id].astro?astro&type=script&index=0&lang.ts")} </main> ` })}`;
}, "C:/Users/richa/OneDrive/Desktop/MMA-Scheduler/frontend/src/pages/fighters/[id].astro", void 0);

const $$file = "C:/Users/richa/OneDrive/Desktop/MMA-Scheduler/frontend/src/pages/fighters/[id].astro";
const $$url = "/fighters/[id]";

const _page = /*#__PURE__*/Object.freeze(/*#__PURE__*/Object.defineProperty({
  __proto__: null,
  default: $$id,
  file: $$file,
  url: $$url
}, Symbol.toStringTag, { value: 'Module' }));

const page = () => _page;

export { page };
