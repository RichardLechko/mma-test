import { supabase } from '../../lib/supabase';

export async function GET({ request }) {
  try {
    
    const url = new URL(request.url);
    const offset = parseInt(url.searchParams.get('offset') || '0');
    const limit = parseInt(url.searchParams.get('limit') || '10');

    console.log(`API request: Fetching fighters with offset=${offset}, limit=${limit}`);

    
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

    
    if (fighters && fighters.length > 0) {
      const names = fighters.map(f => f.name).join(', ');
      console.log(`API response: Fetched ${fighters.length} fighters from offset ${offset}: ${names}`);
    } else {
      console.log(`API response: No fighters found for offset=${offset}, limit=${limit}`);
    }

    
    return new Response(
      JSON.stringify({
        fighters: fighters,
        totalCount: count || 0,
        requestedOffset: offset,
        actualOffset: offset,  
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