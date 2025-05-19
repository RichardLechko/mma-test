export { renderers } from '../../renderers.mjs';

function render({ slots: ___SLOTS___ }) {
		return `<div class="hero-profile__info">
  <div class="hero-profile__tags">
    <p class="hero-profile__tag">Featherweight Division</p>
  </div>
  <p class="hero-profile__nickname">"Bruce Leeroy"</p>
  <h1 class="hero-profile__name">Alex Caceres</h1>
  <div class="hero-profile__division">
    <p class="hero-profile__division-title">Featherweight Division</p>
    <p class="hero-profile__division-body">21-15-0 (W-L-D)</p>
  </div>
  <div class="hero-profile__stats">
    <div class="hero-profile__stat">
      <p class="hero-profile__stat-numb">4</p>
      <p class="hero-profile__stat-text">Wins by Knockout</p>
    </div>
    <div class="hero-profile__stat">
      <p class="hero-profile__stat-numb">7</p>
      <p class="hero-profile__stat-text">Wins by Submission</p>
    </div>
  </div>
</div>
`
	}
render["astro:html"] = true;

const _page = /*#__PURE__*/Object.freeze(/*#__PURE__*/Object.defineProperty({
  __proto__: null,
  default: render
}, Symbol.toStringTag, { value: 'Module' }));

const page = () => _page;

export { page };
