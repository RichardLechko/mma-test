import type { APIRoute } from 'astro';

export const GET: APIRoute = async () => {
  try {
    const uniqueNationalities = [
      'Australia',
      'Brazil',
      'Cameroon',
      'Canada',
      'China',
      'France',
      'Georgia',
      'Ireland',
      'Jamaica',
      'Japan',
      'Kazakhstan',
      'Kyrgyzstan',
      'Mexico',
      'Netherlands',
      'New Zealand',
      'Nigeria',
      'Poland',
      'Russia',
      'South Africa',
      'South Korea',
      'Spain',
      'Sweden',
      'Thailand',
      'United Kingdom',
      'United States'
    ];

    const uniqueWeightClasses = [
      'Flyweight',
      'Bantamweight',
      'Featherweight',
      'Lightweight',
      'Welterweight',
      'Middleweight',
      'Light Heavyweight',
      'Heavyweight',
      "Women's Strawweight",
      "Women's Flyweight",
      "Women's Bantamweight",
      "Women's Featherweight"
    ];

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
