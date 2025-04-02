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