// src/pages/api/fighters.ts
import type { APIRoute } from 'astro';
import { supabase } from '../../lib/supabase';

export const GET: APIRoute = async ({ url }) => {
  try {
    // Get parameters
    const offset = parseInt(url.searchParams.get('offset') || '0');
    const limit = parseInt(url.searchParams.get('limit') || '10');
    const searchTerm = url.searchParams.get('search') || '';
    const status = url.searchParams.get('status') || '';
    const isChampion = url.searchParams.get('champion') === 'true';
    const weightClasses = url.searchParams.getAll('weightClass');
    const nationalities = url.searchParams.getAll('nationality');

    // Build query
    let query = supabase
      .from('fighters')
      .select('id, name, weight_class, nationality, wins, losses, draws, rank, status, no_contests', {
        count: 'exact',
      });

    // Simple filters
    if (searchTerm) query = query.ilike('name', `%${searchTerm}%`);
    
    if (status === 'Retired') {
      query = query.or('status.eq.Retired,status.eq.Not Fighting');
    } else if (status) {
      query = query.eq('status', status);
    }
    
    if (isChampion) query = query.eq('rank', 'Champion');
    
    // Weight classes filter
    if (weightClasses.length > 0) query = query.in('weight_class', weightClasses);
    
    // Nationality filter - using simple approach first
    if (nationalities.length === 1) {
      // If only one nationality, use direct equality comparison
      query = query.eq('nationality', nationalities[0]);
    } else if (nationalities.length > 1) {
      // If multiple nationalities, use OR conditions
      const filters = nationalities.map(nat => `nationality.eq.${nat}`).join(',');
      query = query.or(filters);
    }

    // Execute query
    const { data: fighters, error, count } = await query
      .order('name', { ascending: true })
      .range(offset, offset + limit - 1);

    if (error) throw error;

    // Return results
    return new Response(JSON.stringify({
      fighters: fighters || [],
      count: count || 0,
    }), {
      headers: { 'Content-Type': 'application/json' },
    });
  } catch (error) {
    // Simple error handling
    const errorMessage = error instanceof Error ? error.message : 'Unknown error';
    return new Response(JSON.stringify({ error: errorMessage }), {
      status: 500,
      headers: { 'Content-Type': 'application/json' },
    });
  }
};
