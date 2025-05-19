import type { APIRoute } from 'astro';
import { supabase } from '../../lib/supabase';

export const GET: APIRoute = async () => {
  try {
    const { data: nationalities, error: natError } = await supabase
      .from('fighters')
      .select('nationality')
      .not('nationality', 'is', null)
      .order('nationality');

    const { data: weightClasses, error: wcError } = await supabase
      .from('fighters')
      .select('weight_class')
      .not('weight_class', 'is', null)
      .order('weight_class');

    if (natError || wcError) {
      return new Response(JSON.stringify({ error: natError?.message || wcError?.message }), {
        status: 500,
        headers: { 'Content-Type': 'application/json' }
      });
    }

    const uniqueNationalities = Array.from(
      new Set(
        (nationalities || [])
          .map(item => item.nationality)
          .filter(Boolean)
      )
    ).sort();

    const uniqueWeightClasses = Array.from(
      new Set(
        (weightClasses || [])
          .map(item => item.weight_class)
          .filter(Boolean)
      )
    ).sort();

    return new Response(JSON.stringify({
      nationalities: uniqueNationalities,
      weightClasses: uniqueWeightClasses
    }), {
      headers: { 'Content-Type': 'application/json' }
    });
  } catch (error) {
    return new Response(JSON.stringify({ error: 'Failed to fetch filter options' }), {
      status: 500,
      headers: { 'Content-Type': 'application/json' }
    });
  }
};