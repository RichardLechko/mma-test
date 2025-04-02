import { supabase } from '../../lib/supabase';

// This is a debugging endpoint to help troubleshoot the fighter pagination
export async function GET({ request }) {
  try {
    // Parse URL to get query parameters
    const url = new URL(request.url);
    const testRanges = [
      { start: 0, end: 9 },    // First 10
      { start: 10, end: 19 },  // Next 10
      { start: 20, end: 29 }   // Next 10 after that
    ];
    
    const results = {};
    
    // Fetch multiple ranges to see what they return
    for (const range of testRanges) {
      const { data: fighters } = await supabase
        .from('fighters')
        .select('name, id')
        .order('name', { ascending: true })
        .range(range.start, range.end);
        
      if (fighters) {
        results[`${range.start}-${range.end}`] = fighters.map(f => f.name);
      }
    }
    
    // Return the debug info
    return new Response(
      JSON.stringify({
        message: "This is a debugging endpoint to check pagination",
        ranges: results
      }),
      {
        status: 200,
        headers: {
          'Content-Type': 'application/json'
        }
      }
    );
  } catch (error) {
    console.error('Debug API error:', error);
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