import { s as supabase } from '../../chunks/supabase_CyFqHHS4.mjs';
export { renderers } from '../../renderers.mjs';

const GET = async ({ url }) => {
  const offset = parseInt(url.searchParams.get("offset") || "0");
  const limit = parseInt(url.searchParams.get("limit") || "10");
  const searchTerm = url.searchParams.get("search") || "";
  const status = url.searchParams.get("status") || "";
  const isChampion = url.searchParams.get("champion") === "true";
  const weightClasses = url.searchParams.getAll("weightClass");
  const nationalities = url.searchParams.getAll("nationality");
  try {
    let query = supabase.from("fighters").select(
      "id, name, weight_class, nationality, wins, losses, draws, rank, status, no_contests",
      {
        count: "exact"
      }
    );
    if (searchTerm) {
      query = query.ilike("name", `%${searchTerm}%`);
    }
    if (status) {
      if (status === "Retired") {
        query = query.or("status.eq.Retired,status.eq.Not Fighting");
      } else {
        query = query.eq("status", status);
      }
    }
    if (isChampion) {
      query = query.eq("rank", "Champion");
    }
    if (weightClasses.length > 0) {
      query = query.in("weight_class", weightClasses);
    }
    if (nationalities.length > 0) {
      query = query.in("nationality", nationalities);
    }
    const {
      data: fighters,
      error,
      count
    } = await query.order("name", { ascending: true }).range(offset, offset + limit - 1);
    if (error) {
      return new Response(JSON.stringify({ error: error.message }), {
        status: 500,
        headers: { "Content-Type": "application/json" }
      });
    }
    return new Response(
      JSON.stringify({
        fighters: fighters || [],
        count: count || 0
      }),
      {
        headers: { "Content-Type": "application/json" }
      }
    );
  } catch (error) {
    return new Response(JSON.stringify({ error: "Failed to fetch fighters" }), {
      status: 500,
      headers: { "Content-Type": "application/json" }
    });
  }
};

const _page = /*#__PURE__*/Object.freeze(/*#__PURE__*/Object.defineProperty({
  __proto__: null,
  GET
}, Symbol.toStringTag, { value: 'Module' }));

const page = () => _page;

export { page };
