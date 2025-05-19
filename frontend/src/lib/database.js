import { supabase } from './supabase';

// Fighters related queries
export async function getFighters(limit = 100, offset = 0) {
  const { data, error } = await supabase
    .from('fighters')
    .select('*')
    .order('rank', { ascending: true })
    .range(offset, offset + limit - 1);
  
  if (error) throw error;
  return data;
}

export async function getFighterById(id) {
  const { data, error } = await supabase
    .from('fighters')
    .select(`
      *,
      fighter_rankings(*)
    `)
    .eq('id', id)
    .single();
  
  if (error) throw error;
  return data;
}

export async function getFightersByWeightClass(weightClass, limit = 20) {
  const { data, error } = await supabase
    .from('fighters')
    .select('*')
    .eq('weight_class', weightClass)
    .order('rank', { ascending: true })
    .limit(limit);
  
  if (error) throw error;
  return data;
}

// Events related queries
export async function getUpcomingEvents(limit = 5) {
  const today = new Date().toISOString();
  
  const { data, error } = await supabase
    .from('events')
    .select('*')
    .gte('event_date', today)
    .order('event_date', { ascending: true })
    .limit(limit);
  
  if (error) throw error;
  return data;
}

export async function getEventById(id) {
  const { data, error } = await supabase
    .from('events')
    .select(`
      *,
      fights(
        *,
        fighter1:fighters!fights_fighter1_id_fkey(*),
        fighter2:fighters!fights_fighter2_id_fkey(*)
      )
    `)
    .eq('id', id)
    .single();
  
  if (error) throw error;
  return data;
}

// Fights related queries
export async function getFightsByEventId(eventId) {
  const { data, error } = await supabase
    .from('fights')
    .select(`
      *,
      fighter1:fighters!fights_fighter1_id_fkey(*),
      fighter2:fighters!fights_fighter2_id_fkey(*)
    `)
    .eq('event_id', eventId)
    .order('fight_order', { ascending: true });
  
  if (error) throw error;
  return data;
}

// src/lib/database.js - add this function

/**
 * Get events for a specific year with efficient loading
 * @param {number} year - The year to get events for
 * @param {number} limit - Max events to return
 * @param {number} offset - Offset for pagination
 * @returns {Promise<Array>} - Events for the year
 */
export async function getEventsByYear(year, limit = 10, offset = 0) {
  // Create date range for the year
  const startDate = new Date(year, 0, 1).toISOString();
  const endDate = new Date(year, 11, 31, 23, 59, 59).toISOString();
  
  // Get only necessary data
  const { data, error } = await supabase
    .from('events')
    .select('id, name, event_date, venue, city, country, status')
    .gte('event_date', startDate)
    .lte('event_date', endDate)
    .order('event_date', { ascending: true })
    .range(offset, offset + limit - 1);
  
  if (error) throw error;
  return data || [];
}

/**
 * Get count of events for a specific year
 * @param {number} year - The year to get count for
 * @returns {Promise<number>} - Count of events
 */
export async function getEventCountForYear(year) {
  // Create date range for the year
  const startDate = new Date(year, 0, 1).toISOString();
  const endDate = new Date(year, 11, 31, 23, 59, 59).toISOString();
  
  // Get count only using head query
  const { count, error } = await supabase
    .from('events')
    .select('id', { count: 'exact', head: true })
    .gte('event_date', startDate)
    .lte('event_date', endDate);
  
  if (error) throw error;
  return count || 0;
}

/**
 * Get all available years that have events
 * @returns {Promise<Array>} - Array of years
 */
export async function getAvailableYears() {
  // Execute RPC or use a more efficient query
  // This query is expensive, should be cached
  const { data, error } = await supabase
    .from('events')
    .select('event_date');
  
  if (error) throw error;
  
  // Extract years client-side
  const years = data
    .map(event => {
      const date = new Date(event.event_date);
      return !isNaN(date.getTime()) ? date.getFullYear() : null;
    })
    .filter(Boolean);
  
  // Get unique years and sort
  return [...new Set(years)].sort((a, b) => b - a);
}
