import { s as supabase } from '../../chunks/supabase_CyFqHHS4.mjs';
export { renderers } from '../../renderers.mjs';

async function GET({ request }) {
  try {
    // Parse URL to get query parameters
    const url = new URL(request.url);
    const offset = parseInt(url.searchParams.get('offset') || '0');
    const limit = parseInt(url.searchParams.get('limit') || '10');

    console.log(`API request: Fetching fighters with offset=${offset}, limit=${limit}`);

    // Fetch fighters with pagination - use explicit start and end indices
    const { data: fighters, error, count } = await supabase
      .from('fighters')
      .select('*', { count: 'exact' })
      .order('name', { ascending: true })
      .range(offset, offset + limit - 1);

    if (error) {
      console.error('API error:', error);
      return new Response(
        JSON.stringify({
          error: error.message
        }),
        {
          status: 500,
          headers: {
            'Content-Type': 'application/json'
          }
        }
      );
    }

    // Log fighter names for debugging
    if (fighters && fighters.length > 0) {
      const names = fighters.map(f => f.name).join(', ');
      console.log(`API response: Fetched ${fighters.length} fighters from offset ${offset}: ${names}`);
    } else {
      console.log(`API response: No fighters found for offset=${offset}, limit=${limit}`);
    }

    // Return fighters data
    return new Response(
      JSON.stringify({
        fighters: fighters,
        totalCount: count || 0,
        requestedOffset: offset,  // Reflect back the requested offset
        actualOffset: offset,     // The actual offset used
        limit: limit
      }),
      {
        status: 200,
        headers: {
          'Content-Type': 'application/json'
        }
      }
    );
  } catch (error) {
    console.error('API error:', error);
    return new Response(
      JSON.stringify({
        error: error.message || 'Unknown error'
      }),
      {
        status: 500,
        headers: {
          'Content-Type': 'application/json'
        }
      }
    );
  }
}

const _page = /*#__PURE__*/Object.freeze(/*#__PURE__*/Object.defineProperty({
  __proto__: null,
  GET
}, Symbol.toStringTag, { value: 'Module' }));

const page = () => _page;

export { page };
