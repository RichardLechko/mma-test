import { c as createComponent, m as maybeRenderHead, r as renderTemplate, a as createAstro, b as addAttribute, g as renderHead, e as renderComponent, h as renderSlot } from './astro/server_C_1jQ3jI.mjs';
import 'kleur/colors';
import 'clsx';
/* empty css                          */

const $$Navbar = createComponent(($$result, $$props, $$slots) => {
  return renderTemplate`${maybeRenderHead()}<nav class="navbar"> <div class="navbar-container"> <a href="/" class="navbar-logo">MMA Scheduler</a> <div class="navbar-links"> <a href="/" class="navbar-link">Home</a> <a href="/events" class="navbar-link">Events</a> <a href="/fighters" class="navbar-link">Fighters</a> <a href="/rankings" class="navbar-link">Rankings</a> </div> </div> </nav>`;
}, "C:/Users/richa/OneDrive/Desktop/MMA-Scheduler/frontend/src/components/Navbar.astro", void 0);

const $$Footer = createComponent(($$result, $$props, $$slots) => {
  const currentYear = (/* @__PURE__ */ new Date()).getFullYear();
  return renderTemplate`${maybeRenderHead()}<footer class="footer"> <div class="footer-container"> <div class="footer-content"> <div class="footer-logo">MMA Scheduler</div> <div class="footer-links"> <a href="/" class="footer-link">Home</a> <a href="/events" class="footer-link">Events</a> <a href="/fighters" class="footer-link">Fighters</a> <a href="/rankings" class="footer-link">Rankings</a> </div> </div> <div class="footer-bottom"> <p>&copy; ${currentYear} MMA Scheduler. All rights reserved.</p> </div> </div> </footer>`;
}, "C:/Users/richa/OneDrive/Desktop/MMA-Scheduler/frontend/src/components/Footer.astro", void 0);

const $$Astro = createAstro();
const $$Layout = createComponent(($$result, $$props, $$slots) => {
  const Astro2 = $$result.createAstro($$Astro, $$props, $$slots);
  Astro2.self = $$Layout;
  const { title } = Astro2.props;
  return renderTemplate`<html lang="en"> <head><meta charset="UTF-8"><meta name="viewport" content="width=device-width"><link rel="icon" type="image/svg+xml" href="/favicon.svg"><meta name="generator"${addAttribute(Astro2.generator, "content")}><title>${title}</title>${renderHead()}</head> <body> <div class="site-wrapper"> <header> ${renderComponent($$result, "Navbar", $$Navbar, {})} </header> <main class="site-content"> ${renderSlot($$result, $$slots["default"])} </main> ${renderComponent($$result, "Footer", $$Footer, {})} </div> </body></html>`;
}, "C:/Users/richa/OneDrive/Desktop/MMA-Scheduler/frontend/src/layouts/Layout.astro", void 0);

export { $$Layout as $ };
