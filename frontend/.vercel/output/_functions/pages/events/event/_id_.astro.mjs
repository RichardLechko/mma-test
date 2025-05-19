import { c as createComponent, a as createAstro, e as renderComponent, f as renderScript, r as renderTemplate, m as maybeRenderHead, F as Fragment, b as addAttribute } from '../../../chunks/astro/server_C_1jQ3jI.mjs';
import 'kleur/colors';
import { $ as $$Layout } from '../../../chunks/Layout_BZw4Q8am.mjs';
import { s as supabase } from '../../../chunks/supabase_CyFqHHS4.mjs';
export { renderers } from '../../../renderers.mjs';

const $$Astro = createAstro();
const $$id = createComponent(async ($$result, $$props, $$slots) => {
  const Astro2 = $$result.createAstro($$Astro, $$props, $$slots);
  Astro2.self = $$id;
  const { id } = Astro2.params;
  const eventId = id;
  if (!eventId) {
    return Astro2.redirect("/events");
  }
  const [eventResult, fightsResult] = await Promise.all([
    // Query 1: Fetch only needed event fields - added ufc_url to the select
    supabase.from("events").select(
      "id, name, event_date, venue, city, country, status, ufc_url, attendance"
    ).eq("id", eventId).single(),
    // Query 2: Fetch fights with only needed fighter data
    supabase.from("fights").select(
      `
      id, event_id, fighter1_id, fighter2_id, fighter1_name, fighter2_name,
      fighter1_rank, fighter2_rank, weight_class, is_main_event, 
      fighter1_was_champion, fighter2_was_champion, was_title_fight,
      fight_order, winner_id, result_method, result_method_details, 
      result_round, result_time,
      fighter1:fighter1_id(id, name),
      fighter2:fighter2_id(id, name),
      winner:winner_id(id, name)
    `
    ).eq("event_id", eventId).order("fight_order", { ascending: true })
  ]);
  if (eventResult.error) {
    console.error("Error fetching event:", eventResult.error);
  }
  if (fightsResult.error) {
    console.error("Error fetching fights:", fightsResult.error);
  }
  const eventData = eventResult.data;
  let fightsList = [];
  if (fightsResult.data) {
    fightsList = fightsResult.data;
  }
  const isEventCompleted = eventData?.status === "Completed";
  if (isEventCompleted) {
    Astro2.response.headers.set("Cache-Control", "public, max-age=86400");
  } else {
    Astro2.response.headers.set("Cache-Control", "public, max-age=3600");
  }
  const isFightCanceled = (fight, isCompleted) => {
    return isCompleted && !fight.winner_id && !fight.result_method && fight.result_method !== "Draw" && fight.result_method !== "No Contest";
  };
  return renderTemplate`${renderComponent($$result, "Layout", $$Layout, { "title": eventData ? eventData.name : "Event Details" }, { "default": async ($$result2) => renderTemplate` ${maybeRenderHead()}<main class="event-page"> ${eventData ? renderTemplate`<div class="event-details-container"> <div class="event-header">  <div class="event-countdown"> ${(() => {
    const eventDate = new Date(eventData.event_date);
    if (isNaN(eventDate.getTime())) {
      return renderTemplate`<div>Date not available</div>`;
    }
    const utcYear = eventDate.getUTCFullYear();
    const utcMonth = eventDate.getUTCMonth();
    const utcDay = eventDate.getUTCDate();
    const displayDate = new Date(utcYear, utcMonth, utcDay);
    const today = /* @__PURE__ */ new Date();
    const todayAtMidnight = new Date(
      today.getFullYear(),
      today.getMonth(),
      today.getDate()
    );
    const isPastEvent = displayDate < todayAtMidnight;
    const calculateDaysDifference = (date1, date2) => {
      const d1 = new Date(
        date1.getFullYear(),
        date1.getMonth(),
        date1.getDate()
      );
      const d2 = new Date(
        date2.getFullYear(),
        date2.getMonth(),
        date2.getDate()
      );
      const timeDiff = Math.abs(d2.getTime() - d1.getTime());
      return Math.round(timeDiff / (1e3 * 60 * 60 * 24));
    };
    const diffDays = calculateDaysDifference(
      displayDate,
      todayAtMidnight
    );
    let timeDisplay;
    if (!isPastEvent) {
      if (diffDays === 0) {
        timeDisplay = "Today";
      } else if (diffDays === 1) {
        timeDisplay = "Tomorrow";
      } else {
        timeDisplay = `${diffDays} days until event`;
      }
    } else {
      if (diffDays === 0) {
        timeDisplay = "Today";
      } else if (diffDays === 1) {
        timeDisplay = "Yesterday";
      } else {
        timeDisplay = `${diffDays} days ago`;
      }
    }
    return renderTemplate`${renderComponent($$result2, "Fragment", Fragment, {}, { "default": async ($$result3) => renderTemplate` <span class="countdown-value">${timeDisplay}</span> <span class="countdown-label">${eventData.status}</span> ` })}`;
  })()} </div> <h1>${eventData.name}</h1>  ${eventData.ufc_url && renderTemplate`<a${addAttribute(eventData.ufc_url, "href")} target="_blank" rel="noopener noreferrer" class="ufc-link"> <span class="ufc-icon">UFC</span> <span class="link-text">Official Page</span> </a>`} <div class="event-meta"> <div class="event-date-time"> ${(() => {
    const eventDate = new Date(eventData.event_date);
    if (isNaN(eventDate.getTime())) {
      return renderTemplate`<div>Date not available</div>`;
    }
    const formattedDate = eventDate.toLocaleDateString("en-US", {
      weekday: "long",
      year: "numeric",
      month: "long",
      day: "numeric"
    });
    const formattedTime = eventDate.toLocaleTimeString("en-US", {
      hour: "2-digit",
      minute: "2-digit",
      timeZoneName: "short"
    });
    return renderTemplate`${renderComponent($$result2, "Fragment", Fragment, {}, { "default": async ($$result3) => renderTemplate` <div class="event-date">${formattedDate}</div> <div class="event-time">${formattedTime}</div> ` })}`;
  })()} </div> <div class="event-location"> ${eventData.venue && renderTemplate`<span class="venue">${eventData.venue}</span>`} ${eventData.city && eventData.country && renderTemplate`<span class="location"> ${eventData.city}, ${eventData.country} </span>`}  ${eventData.attendance && isEventCompleted && renderTemplate`<span class="attendance"> <span class="attendance-icon">👥</span> <span class="attendance-count"> ${(() => {
    const attendanceStr = String(eventData.attendance);
    const numericValue = attendanceStr.replace(
      /[^\d]/g,
      ""
    );
    const formattedAttendance = numericValue ? new Intl.NumberFormat().format(
      parseInt(numericValue)
    ) : attendanceStr;
    return `${formattedAttendance} fans`;
  })()} </span> </span>`} </div> </div> <div class="event-countdown"> ${(() => {
    const eventDate = new Date(eventData.event_date);
    if (isNaN(eventDate.getTime())) {
      return renderTemplate`<div>Date not available</div>`;
    }
    const utcYear = eventDate.getUTCFullYear();
    const utcMonth = eventDate.getUTCMonth();
    const utcDay = eventDate.getUTCDate();
    const displayDate = new Date(utcYear, utcMonth, utcDay);
    const today = /* @__PURE__ */ new Date();
    const todayAtMidnight = new Date(
      today.getFullYear(),
      today.getMonth(),
      today.getDate()
    );
    const isPastEvent = displayDate < todayAtMidnight;
    const calculateDaysDifference = (date1, date2) => {
      const d1 = new Date(
        date1.getFullYear(),
        date1.getMonth(),
        date1.getDate()
      );
      const d2 = new Date(
        date2.getFullYear(),
        date2.getMonth(),
        date2.getDate()
      );
      const timeDiff = Math.abs(d2.getTime() - d1.getTime());
      return Math.round(timeDiff / (1e3 * 60 * 60 * 24));
    };
    const diffDays = calculateDaysDifference(
      displayDate,
      todayAtMidnight
    );
    let timeDisplay;
    if (!isPastEvent) {
      if (diffDays === 0) {
        timeDisplay = "Today";
      } else if (diffDays === 1) {
        timeDisplay = "Tomorrow";
      } else {
        timeDisplay = `${diffDays} days until event`;
      }
    } else {
      if (diffDays === 0) {
        timeDisplay = "Today";
      } else if (diffDays === 1) {
        timeDisplay = "Yesterday";
      } else {
        timeDisplay = `${diffDays} days ago`;
      }
    }
    return renderTemplate`${renderComponent($$result2, "Fragment", Fragment, {}, { "default": async ($$result3) => renderTemplate` <span class="countdown-value">${timeDisplay}</span> <span class="countdown-label">${eventData.status}</span> ` })}`;
  })()} </div> </div> <section class="fight-card"> <h2>Fight Card</h2> ${fightsList.length > 0 ? renderTemplate`<div class="fights-list"> ${fightsList.map((fight) => {
    const isCanceled = isFightCanceled(fight, isEventCompleted);
    return renderTemplate`<div${addAttribute(`fight ${fight.is_main_event ? "main-event" : ""} ${fight.was_title_fight ? "title-fight" : ""} ${isCanceled ? "canceled-fight" : ""}`, "class")}> <div> ${fight.is_main_event && renderTemplate`<span class="main-event-tag">Main Event</span>`} ${fight.was_title_fight && renderTemplate`<span class="title-fight-tag">Title Fight</span>`} ${isCanceled && renderTemplate`<span class="canceled-tag">Canceled</span>`} <span class="weight-class"> ${fight.weight_class} ${fight.was_title_fight && " Championship"} </span> </div> <div class="fighters"> <a${addAttribute(`/fighters/${fight.fighter1.id}`, "href")}${addAttribute(`fighter fighter-1 ${fight.winner_id === fight.fighter1_id ? "winner" : ""} ${isCanceled ? "canceled" : ""}`, "class")}> <div class="fighter-name"> <span class="name">${fight.fighter1.name}</span> </div> <div class="fighter-status">  ${fight.fighter1_was_champion ? renderTemplate`<div class="champion-badge">C</div>` : fight.fighter1_rank && fight.fighter1_rank !== "NR" ? (
      /* If they had a rank, show it */
      renderTemplate`<div class="fighter-rank">
#${fight.fighter1_rank} </div>`
    ) : (
      /* Only show Unranked if the other fighter has a rank or is champion */
      (fight.fighter2_was_champion || fight.fighter2_rank && fight.fighter2_rank !== "NR") && renderTemplate`<div class="fighter-unranked">Unranked</div>`
    )} </div> </a> <div class="vs">VS</div> <a${addAttribute(`/fighters/${fight.fighter2.id}`, "href")}${addAttribute(`fighter fighter-2 ${fight.winner_id === fight.fighter2_id ? "winner" : ""} ${isCanceled ? "canceled" : ""}`, "class")}> <div class="fighter-name"> <span class="name">${fight.fighter2.name}</span> </div> <div class="fighter-status">  ${fight.fighter2_was_champion ? renderTemplate`<div class="champion-badge">C</div>` : fight.fighter2_rank && fight.fighter2_rank !== "NR" ? (
      /* If they had a rank, show it */
      renderTemplate`<div class="fighter-rank">
#${fight.fighter2_rank} </div>`
    ) : (
      /* Only show Unranked if the other fighter has a rank or is champion */
      (fight.fighter1_was_champion || fight.fighter1_rank && fight.fighter1_rank !== "NR") && renderTemplate`<div class="fighter-unranked">Unranked</div>`
    )} </div> </a> </div>  ${isEventCompleted && renderTemplate`${renderComponent($$result2, "Fragment", Fragment, {}, { "default": async ($$result3) => renderTemplate`${isCanceled ? renderTemplate`<div class="fight-result canceled-result"> <div class="result-header">Fight Canceled</div> </div>` : fight.winner_id ? renderTemplate`<div class="fight-result"> <div class="result-header"> <span class="winner-name"> ${fight.winner?.name} </span>${" "}
wins by${" "} <span class="method"> ${fight.result_method} </span> </div> ${fight.result_method_details && renderTemplate`<div class="result-details"> ${fight.result_method_details} </div>`} ${(fight.result_round || fight.result_time) && renderTemplate`<div class="result-timing"> ${fight.result_round && `Round ${fight.result_round}`} ${fight.result_round && fight.result_time && ` \u2022 `} ${fight.result_time && // Time handling logic
    (() => {
      const timeStr = fight.result_time;
      const halfLength = timeStr.length / 2;
      const firstHalf = timeStr.substring(
        0,
        halfLength
      );
      const secondHalf = timeStr.substring(halfLength);
      return firstHalf === secondHalf ? firstHalf : timeStr;
    })()} </div>`} </div>` : fight.result_method ? renderTemplate`<div class="fight-result draw-result"> <div class="result-header"> <span class="method"> ${fight.result_method} </span> </div> ${(fight.result_round || fight.result_time) && renderTemplate`<div class="result-timing"> ${fight.result_round && `Round ${fight.result_round}`} ${fight.result_round && fight.result_time && ` \u2022 `} ${fight.result_time && // Time handling logic
    (() => {
      const timeStr = fight.result_time;
      const halfLength = timeStr.length / 2;
      const firstHalf = timeStr.substring(
        0,
        halfLength
      );
      const secondHalf = timeStr.substring(halfLength);
      return firstHalf === secondHalf ? firstHalf : timeStr;
    })()} </div>`} </div>` : null}` })}`} </div>`;
  })} </div>` : renderTemplate`<div class="no-fights"> <p>No fights announced yet for this event.</p> </div>`} </section> </div>` : renderTemplate`<div class="event-not-found"> <h1>Event Not Found</h1> <p>
Sorry, the event you're looking for doesn't exist or has been
            removed.
</p> <a href="/events" class="back-button">
Back to Events
</a> </div>`} </main> ` })} ${renderScript($$result, "C:/Users/richa/OneDrive/Desktop/MMA-Scheduler/frontend/src/pages/events/event/[id].astro?astro&type=script&index=0&lang.ts")}`;
}, "C:/Users/richa/OneDrive/Desktop/MMA-Scheduler/frontend/src/pages/events/event/[id].astro", void 0);

const $$file = "C:/Users/richa/OneDrive/Desktop/MMA-Scheduler/frontend/src/pages/events/event/[id].astro";
const $$url = "/events/event/[id]";

const _page = /*#__PURE__*/Object.freeze(/*#__PURE__*/Object.defineProperty({
  __proto__: null,
  default: $$id,
  file: $$file,
  url: $$url
}, Symbol.toStringTag, { value: 'Module' }));

const page = () => _page;

export { page };
