import type { APIRoute } from 'astro';
import { supabase } from '../../lib/supabase';

export const GET: APIRoute = async ({ url }) => {
  const offset = parseInt(url.searchParams.get('offset') || '0');
  const limit = parseInt(url.searchParams.get('limit') || '10');
  
  try {
    const { data: fighters, error } = await supabase
      .from('fighters')
      .select('id, name, weight_class, nationality, wins, losses, draws, rank, status, no_contests')
      .order('name', { ascending: true })
      .range(offset, offset + limit - 1);
    
    if (error) {
      return new Response(JSON.stringify({ error: error.message }), {
        status: 500,
        headers: { 'Content-Type': 'application/json' }
      });
    }
    
    return new Response(JSON.stringify({ fighters: fighters || [] }), {
      headers: { 'Content-Type': 'application/json' }
    });
  } catch (error) {
    return new Response(JSON.stringify({ error: 'Failed to fetch fighters' }), {
      status: 500,
      headers: { 'Content-Type': 'application/json' }
    });
  }
};