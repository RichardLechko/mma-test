// src/pages/api/fighters.ts
import type { APIRoute } from 'astro';
import { supabase } from '../../lib/supabase';

export const GET: APIRoute = async ({ url }) => {
  const offset = parseInt(url.searchParams.get('offset') || '0');
  const limit = parseInt(url.searchParams.get('limit') || '10');
  const searchTerm = url.searchParams.get('search') || '';
  const status = url.searchParams.get('status') || '';
  const isChampion = url.searchParams.get('champion') === 'true';

  // Handle multiple values for these filters
  const weightClasses = url.searchParams.getAll('weightClass');
  const nationalities = url.searchParams.getAll('nationality');

  try {
    let query = supabase
      .from('fighters')
      .select(
        'id, name, weight_class, nationality, wins, losses, draws, rank, status, no_contests',
        {
          count: 'exact',
        },
      );

    // Apply filters if provided
    if (searchTerm) {
      query = query.ilike('name', `%${searchTerm}%`);
    }

    if (status) {
      if (status === 'Retired') {
        // Handle both "Retired" and "Not Fighting" statuses
        query = query.or('status.eq.Retired,status.eq.Not Fighting');
      } else {
        query = query.eq('status', status);
      }
    }

    if (isChampion) {
      query = query.eq('rank', 'Champion');
    }

    // Handle multiple weight classes (OR condition)
    if (weightClasses.length > 0) {
      query = query.in('weight_class', weightClasses);
    }

    // Handle nationality filter - THIS IS THE CRITICAL CHANGE
    if (nationalities.length > 0) {
      // FIX: Instead of using the .in() method, create a manual OR filter for exact equality
      const filters = nationalities.map((nat, index) => {
        return `nationality.eq.${nat}`;
      });
      
      // Apply the OR filter
      query = query.or(filters.join(','));
    }

    // Get the paginated fighters with the filters and count
    const {
      data: fighters,
      error,
      count,
    } = await query
      .order('name', { ascending: true })
      .range(offset, offset + limit - 1);

    if (error) {
      return new Response(JSON.stringify({ error: error.message }), {
        status: 500,
        headers: { 'Content-Type': 'application/json' },
      });
    }

    return new Response(
      JSON.stringify({
        fighters: fighters || [],
        count: count || 0,
      }),
      {
        headers: { 'Content-Type': 'application/json' },
      },
    );
  } catch (error) {
    return new Response(JSON.stringify({ error: 'Failed to fetch fighters' }), {
      status: 500,
      headers: { 'Content-Type': 'application/json' },
    });
  }
};