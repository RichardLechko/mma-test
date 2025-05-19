import { c as createComponent, a as createAstro } from '../chunks/astro/server_C_1jQ3jI.mjs';
import 'kleur/colors';
import 'clsx';
export { renderers } from '../renderers.mjs';

const $$Astro = createAstro();
const $$Index = createComponent(($$result, $$props, $$slots) => {
  const Astro2 = $$result.createAstro($$Astro, $$props, $$slots);
  Astro2.self = $$Index;
  const currentYear = (/* @__PURE__ */ new Date()).getFullYear();
  return Astro2.redirect(`/events/${currentYear}`);
}, "C:/Users/richa/OneDrive/Desktop/MMA-Scheduler/frontend/src/pages/events/index.astro", void 0);

const $$file = "C:/Users/richa/OneDrive/Desktop/MMA-Scheduler/frontend/src/pages/events/index.astro";
const $$url = "/events";

const _page = /*#__PURE__*/Object.freeze(/*#__PURE__*/Object.defineProperty({
	__proto__: null,
	default: $$Index,
	file: $$file,
	url: $$url
}, Symbol.toStringTag, { value: 'Module' }));

const page = () => _page;

export { page };
