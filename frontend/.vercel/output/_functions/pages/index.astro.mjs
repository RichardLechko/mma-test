import { c as createComponent, e as renderComponent, r as renderTemplate, m as maybeRenderHead, b as addAttribute } from '../chunks/astro/server_C_1jQ3jI.mjs';
import 'kleur/colors';
import { $ as $$Layout } from '../chunks/Layout_BZw4Q8am.mjs';
import { s as supabase } from '../chunks/supabase_CyFqHHS4.mjs';
export { renderers } from '../renderers.mjs';

// Events related queries
async function getUpcomingEvents(limit = 5) {
  const today = new Date().toISOString();
  
  const { data, error } = await supabase
    .from('events')
    .select('*')
    .gte('event_date', today)
    .order('event_date', { ascending: true })
    .limit(limit);
  
  if (error) throw error;
  return data;
}

const $$Index = createComponent(async ($$result, $$props, $$slots) => {
  const upcomingEvents = await getUpcomingEvents(4);
  function formatDate(dateString) {
    if (!dateString) return "";
    const date = new Date(dateString);
    if (isNaN(date.getTime())) return "Invalid date";
    const utcYear = date.getUTCFullYear();
    const utcMonth = date.getUTCMonth();
    const utcDay = date.getUTCDate();
    const displayDate = new Date(utcYear, utcMonth, utcDay);
    return displayDate.toLocaleDateString("en-US", {
      weekday: "long",
      year: "numeric",
      month: "long",
      day: "numeric"
    });
  }
  function getDaysUntil(dateString) {
    if (!dateString) return "";
    const eventDate = new Date(dateString);
    if (isNaN(eventDate.getTime())) return "Date unknown";
    const utcYear = eventDate.getUTCFullYear();
    const utcMonth = eventDate.getUTCMonth();
    const utcDay = eventDate.getUTCDate();
    const displayDate = new Date(utcYear, utcMonth, utcDay);
    const today = /* @__PURE__ */ new Date();
    const todayAtMidnight = new Date(today.getFullYear(), today.getMonth(), today.getDate());
    const calculateDaysDifference = (date1, date2) => {
      const d1 = new Date(date1.getFullYear(), date1.getMonth(), date1.getDate());
      const d2 = new Date(date2.getFullYear(), date2.getMonth(), date2.getDate());
      const timeDiff = Math.abs(d2.getTime() - d1.getTime());
      return Math.round(timeDiff / (1e3 * 60 * 60 * 24));
    };
    const diffDays = calculateDaysDifference(displayDate, todayAtMidnight);
    const isPastEvent = displayDate < todayAtMidnight;
    if (isPastEvent) {
      if (diffDays === 0) return "Today";
      if (diffDays === 1) return "Yesterday";
      return `${diffDays} days ago`;
    } else {
      if (diffDays === 0) return "Today";
      if (diffDays === 1) return "Tomorrow";
      return `${diffDays} days away`;
    }
  }
  return renderTemplate`${renderComponent($$result, "Layout", $$Layout, { "title": "MMA Scheduler - UFC Events and Fighter Database" }, { "default": async ($$result2) => renderTemplate` ${maybeRenderHead()}<main class="home"> <section class="hero"> <div class="hero-content"> <h1>MMA Scheduler</h1> <p class="tagline">Your hub for UFC events and fighter information</p> <div class="hero-buttons"> <a href="/events" class="btn btn-primary">View Events</a> <a href="/fighters" class="btn">Browse Fighters</a> <a href="/rankings" class="btn">Rankings</a> </div> </div> </section> <div class="content"> <section class="event-preview card"> <h2>Upcoming UFC Events</h2> <div class="events-grid"> ${upcomingEvents && upcomingEvents.length > 0 ? upcomingEvents.map((event) => renderTemplate`<div class="event-details"> <h3>${event.name}</h3> <p class="event-date">${formatDate(event.event_date)}</p> <p class="event-location"> ${event.venue && `${event.venue}, `} ${event.city && `${event.city}, `} ${event.country} </p> <div class="countdown"> <span>${getDaysUntil(event.event_date)}</span> </div> <a${addAttribute(`/events/event/${event.id}`, "href")} class="btn btn-primary">
View Fight Card
</a> </div>`) : renderTemplate`<p>No upcoming events scheduled at this time.</p>`} </div> </section> <section class="features"> <div class="feature card"> <div class="icon">📅</div> <h3>Event Calendar</h3> <p>Stay up to date with all upcoming UFC events.</p> </div> <div class="feature card"> <div class="icon">👊</div> <h3>Fighter Database</h3> <p>Comprehensive fighter stats and records.</p> </div> <div class="feature card"> <div class="icon">🏆</div> <h3>Rankings</h3> <p>Follow the latest UFC rankings.</p> </div> </section> </div> </main> ` })}`;
}, "C:/Users/richa/OneDrive/Desktop/MMA-Scheduler/frontend/src/pages/index.astro", void 0);

const $$file = "C:/Users/richa/OneDrive/Desktop/MMA-Scheduler/frontend/src/pages/index.astro";
const $$url = "";

const _page = /*#__PURE__*/Object.freeze(/*#__PURE__*/Object.defineProperty({
  __proto__: null,
  default: $$Index,
  file: $$file,
  url: $$url
}, Symbol.toStringTag, { value: 'Module' }));

const page = () => _page;

export { page };
