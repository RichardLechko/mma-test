import type { APIRoute } from 'astro';
import { supabase } from '../../lib/supabase';

export const GET: APIRoute = async ({ url }) => {
  const year = parseInt(url.searchParams.get('year') || '0');
  const offset = parseInt(url.searchParams.get('offset') || '0');
  const limit = parseInt(url.searchParams.get('limit') || '10');
  
  const startOfYear = new Date(year, 0, 1);
  const endOfYear = new Date(year, 11, 31, 23, 59, 59);
  
  const startISO = startOfYear.toISOString();
  const endISO = endOfYear.toISOString();
  
  try {
    const { data: events, error } = await supabase
      .from('events')
      .select('id, name, event_date, venue, city, country, status')
      .gte('event_date', startISO)
      .lte('event_date', endISO)
      .order('event_date', { ascending: true })
      .range(offset, offset + limit - 1);
    
    if (error) {
      return new Response(JSON.stringify({ error: error.message }), {
        status: 500,
        headers: { 'Content-Type': 'application/json' }
      });
    }
    
    const filteredEvents = events?.filter(event => {
      if (!event || !event.event_date) return false;
      const date = new Date(event.event_date);
      return !isNaN(date.getTime());
    });
    
    const { count } = await supabase
      .from('events')
      .select('id', { count: 'exact', head: true })
      .gte('event_date', startISO)
      .lte('event_date', endISO);
    
    const hasMore = (offset + limit) < (count || 0);
    
    return new Response(JSON.stringify({ 
      events: filteredEvents || [],
      hasMore,
      nextOffset: offset + limit
    }), {
      headers: { 
        'Content-Type': 'application/json',
        'Cache-Control': 'public, max-age=3600'
      }
    });
  } catch (error) {
    return new Response(JSON.stringify({ error: 'Failed to fetch events' }), {
      status: 500,
      headers: { 'Content-Type': 'application/json' }
    });
  }
};
