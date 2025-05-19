import { c as createComponent, a as createAstro, e as renderComponent, f as renderScript, r as renderTemplate, m as maybeRenderHead, b as addAttribute } from '../../chunks/astro/server_C_1jQ3jI.mjs';
import 'kleur/colors';
import { $ as $$Layout } from '../../chunks/Layout_BZw4Q8am.mjs';
import { $ as $$FilterDropdown } from '../../chunks/FilterDropdown_CxyROIjD.mjs';
import { s as supabase } from '../../chunks/supabase_CyFqHHS4.mjs';
/* empty css                                     */
export { renderers } from '../../renderers.mjs';

const $$Astro = createAstro();
const $$year = createComponent(async ($$result, $$props, $$slots) => {
  const Astro2 = $$result.createAstro($$Astro, $$props, $$slots);
  Astro2.self = $$year;
  const { year: yearParam } = Astro2.params;
  const yearNum = parseInt(yearParam || (/* @__PURE__ */ new Date()).getFullYear().toString());
  const currentYear = (/* @__PURE__ */ new Date()).getFullYear();
  const validYear = !isNaN(yearNum) ? yearNum : currentYear;
  const isHistoricalYear = validYear < currentYear;
  let availableYears = [validYear.toString()];
  try {
    const { data, error } = await supabase.from("events").select("event_date");
    if (!error && data) {
      const years = data.map((event) => {
        const date = new Date(event.event_date);
        return !isNaN(date.getTime()) ? date.getFullYear() : null;
      }).filter((year) => year !== null);
      if (!years.includes(currentYear)) {
        years.push(currentYear);
      }
      availableYears = [...new Set(years)].sort((a, b) => b - a).map((y) => y.toString());
    }
  } catch (error) {
    console.error("Error fetching years:", error);
  }
  const yearOptions = availableYears.map((year) => ({
    value: year,
    label: year
  }));
  return renderTemplate`${renderComponent($$result, "Layout", $$Layout, { "title": `UFC Events - ${validYear}`, "data-astro-cid-awfrf6r4": true }, { "default": async ($$result2) => renderTemplate` ${maybeRenderHead()}<main class="events-page" data-astro-cid-awfrf6r4> <section class="events-container" data-astro-cid-awfrf6r4> <div class="events-header" data-astro-cid-awfrf6r4> <h1 data-astro-cid-awfrf6r4>UFC Events ${validYear}</h1> <!-- Year filter dropdown using your FilterDropdown component --> ${renderComponent($$result2, "FilterDropdown", $$FilterDropdown, { "label": "Year", "options": yearOptions, "currentValue": validYear.toString(), "id": "year-selector", "data-astro-cid-awfrf6r4": true })} <!-- Hidden select for the redirect functionality --> <select id="year-selector" class="hidden-select" data-astro-cid-awfrf6r4> ${availableYears.map((year) => renderTemplate`<option${addAttribute(year, "value")}${addAttribute(year === validYear.toString(), "selected")} data-astro-cid-awfrf6r4> ${year} </option>`)} </select> </div> <div id="events-container"${addAttribute(validYear, "data-year")}${addAttribute(isHistoricalYear ? "false" : "true", "data-is-current-year")} data-astro-cid-awfrf6r4> <div class="loading-container" data-astro-cid-awfrf6r4> <div class="loading-spinner" data-astro-cid-awfrf6r4></div> <p data-astro-cid-awfrf6r4>Loading events for ${validYear}...</p> </div> </div> </section> </main> ` })} ${renderScript($$result, "C:/Users/richa/OneDrive/Desktop/MMA-Scheduler/frontend/src/pages/events/[year].astro?astro&type=script&index=0&lang.ts")} `;
}, "C:/Users/richa/OneDrive/Desktop/MMA-Scheduler/frontend/src/pages/events/[year].astro", void 0);

const $$file = "C:/Users/richa/OneDrive/Desktop/MMA-Scheduler/frontend/src/pages/events/[year].astro";
const $$url = "/events/[year]";

const _page = /*#__PURE__*/Object.freeze(/*#__PURE__*/Object.defineProperty({
  __proto__: null,
  default: $$year,
  file: $$file,
  url: $$url
}, Symbol.toStringTag, { value: 'Module' }));

const page = () => _page;

export { page };
