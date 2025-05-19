import { renderers } from './renderers.mjs';
import { a as actions } from './chunks/_noop-actions_CfKMStZn.mjs';
import { c as createExports } from './chunks/entrypoint_BHqA98pV.mjs';
import { manifest } from './manifest_BFPnVVJt.mjs';

const serverIslandMap = new Map();;

const _page0 = () => import('./pages/_image.astro.mjs');
const _page1 = () => import('./pages/api/events.astro.mjs');
const _page2 = () => import('./pages/api/fighters.astro.mjs');
const _page3 = () => import('./pages/api/fighters.json.astro.mjs');
const _page4 = () => import('./pages/api/filter-options.astro.mjs');
const _page5 = () => import('./pages/events/event/_id_.astro.mjs');
const _page6 = () => import('./pages/events/_year_.astro.mjs');
const _page7 = () => import('./pages/events.astro.mjs');
const _page8 = () => import('./pages/fighters/test.astro.mjs');
const _page9 = () => import('./pages/fighters/_id_.astro.mjs');
const _page10 = () => import('./pages/fighters.astro.mjs');
const _page11 = () => import('./pages/rankings.astro.mjs');
const _page12 = () => import('./pages/index.astro.mjs');
const pageMap = new Map([
    ["node_modules/astro/dist/assets/endpoint/generic.js", _page0],
    ["src/pages/api/events.ts", _page1],
    ["src/pages/api/fighters.ts", _page2],
    ["src/pages/api/fighters.json.js", _page3],
    ["src/pages/api/filter-options.ts", _page4],
    ["src/pages/events/event/[id].astro", _page5],
    ["src/pages/events/[year].astro", _page6],
    ["src/pages/events/index.astro", _page7],
    ["src/pages/fighters/test.html", _page8],
    ["src/pages/fighters/[id].astro", _page9],
    ["src/pages/fighters/index.astro", _page10],
    ["src/pages/rankings/index.astro", _page11],
    ["src/pages/index.astro", _page12]
]);

const _manifest = Object.assign(manifest, {
    pageMap,
    serverIslandMap,
    renderers,
    actions,
    middleware: () => import('./_noop-middleware.mjs')
});
const _args = {
    "middlewareSecret": "f179cbae-3566-4025-839e-708b7ee27084",
    "skewProtection": false
};
const _exports = createExports(_manifest, _args);
const __astrojsSsrVirtualEntry = _exports.default;

export { __astrojsSsrVirtualEntry as default, pageMap };
