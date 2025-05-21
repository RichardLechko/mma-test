import type { APIRoute } from 'astro';

export const GET: APIRoute = async ({ url }) => {
  const year = parseInt(url.searchParams.get('year') || '0');
  const offset = parseInt(url.searchParams.get('offset') || '0');
  const limit = parseInt(url.searchParams.get('limit') || '10');
 
  const startOfYear = new Date(year, 0, 1);
  const endOfYear = new Date(year, 11, 31, 23, 59, 59);
 
  try {
    const allEvents = [
      {
        id: "1",
        name: "UFC 300",
        event_date: "2024-04-13T22:00:00.000Z",
        venue: "T-Mobile Arena",
        city: "Las Vegas",
        country: "USA",
        status: "Completed"
      },
      {
        id: "2",
        name: "UFC 301",
        event_date: "2024-05-04T22:00:00.000Z",
        venue: "Rio Arena",
        city: "Rio de Janeiro",
        country: "Brazil",
        status: "Completed"
      },
      {
        id: "3",
        name: "UFC Fight Night",
        event_date: "2024-05-11T19:00:00.000Z",
        venue: "Enterprise Center",
        city: "St. Louis",
        country: "USA",
        status: "Completed"
      },
      {
        id: "4",
        name: "UFC Fight Night",
        event_date: "2024-05-18T19:00:00.000Z",
        venue: "UFC Apex",
        city: "Las Vegas",
        country: "USA",
        status: "Completed"
      },
      {
        id: "5",
        name: "UFC 302",
        event_date: "2024-06-01T22:00:00.000Z",
        venue: "Prudential Center",
        city: "Newark",
        country: "USA",
        status: "Completed"
      },
      {
        id: "6",
        name: "UFC Fight Night",
        event_date: "2024-06-15T19:00:00.000Z",
        venue: "UFC Apex",
        city: "Las Vegas",
        country: "USA",
        status: "Scheduled"
      },
      {
        id: "7",
        name: "UFC 303",
        event_date: "2024-06-29T22:00:00.000Z",
        venue: "T-Mobile Arena",
        city: "Las Vegas",
        country: "USA",
        status: "Scheduled"
      },
      {
        id: "8",
        name: "UFC International Fight Week",
        event_date: "2024-07-06T19:00:00.000Z",
        venue: "T-Mobile Arena",
        city: "Las Vegas",
        country: "USA",
        status: "Scheduled"
      },
      {
        id: "9",
        name: "UFC 304",
        event_date: "2024-07-27T19:00:00.000Z",
        venue: "Co-op Live",
        city: "Manchester",
        country: "United Kingdom",
        status: "Scheduled"
      },
      {
        id: "10",
        name: "UFC Fight Night",
        event_date: "2024-08-17T19:00:00.000Z",
        venue: "TBD",
        city: "TBD",
        country: "TBD",
        status: "Scheduled"
      },
      {
        id: "11",
        name: "UFC 305",
        event_date: "2024-08-24T22:00:00.000Z",
        venue: "RAC Arena",
        city: "Perth",
        country: "Australia",
        status: "Scheduled"
      },
      {
        id: "12",
        name: "UFC 306",
        event_date: "2024-09-14T22:00:00.000Z",
        venue: "Sphere",
        city: "Las Vegas",
        country: "USA",
        status: "Scheduled"
      },
      {
        id: "13",
        name: "UFC 295",
        event_date: "2023-11-11T22:00:00.000Z",
        venue: "Madison Square Garden",
        city: "New York",
        country: "USA",
        status: "Completed"
      },
      {
        id: "14",
        name: "UFC 296",
        event_date: "2023-12-16T22:00:00.000Z",
        venue: "T-Mobile Arena",
        city: "Las Vegas",
        country: "USA",
        status: "Completed"
      },
      {
        id: "15",
        name: "UFC 297",
        event_date: "2024-01-20T22:00:00.000Z",
        venue: "Scotiabank Arena",
        city: "Toronto",
        country: "Canada",
        status: "Completed"
      },
      {
        id: "16",
        name: "UFC 298",
        event_date: "2024-02-17T22:00:00.000Z",
        venue: "Honda Center",
        city: "Anaheim",
        country: "USA",
        status: "Completed"
      },
      {
        id: "17",
        name: "UFC 299",
        event_date: "2024-03-09T22:00:00.000Z",
        venue: "Kaseya Center",
        city: "Miami",
        country: "USA",
        status: "Completed"
      },
      {
        id: "18",
        name: "UFC 290",
        event_date: "2023-07-08T22:00:00.000Z",
        venue: "T-Mobile Arena",
        city: "Las Vegas",
        country: "USA",
        status: "Completed"
      },
      {
        id: "19",
        name: "UFC 291",
        event_date: "2023-07-29T22:00:00.000Z",
        venue: "Delta Center",
        city: "Salt Lake City",
        country: "USA",
        status: "Completed"
      },
      {
        id: "20",
        name: "UFC 292",
        event_date: "2023-08-19T22:00:00.000Z",
        venue: "TD Garden",
        city: "Boston",
        country: "USA",
        status: "Completed"
      }
    ];
    
    const filteredEvents = allEvents.filter(event => {
      if (!event || !event.event_date) return false;
      const date = new Date(event.event_date);
      const eventYear = date.getFullYear();
      return !isNaN(date.getTime()) && 
             (year === 0 || eventYear === year) &&
             date >= startOfYear && date <= endOfYear;
    });
    
    const count = filteredEvents.length;
    
    const paginatedEvents = filteredEvents.slice(offset, offset + limit);
    
    const hasMore = (offset + limit) < count;
   
    return new Response(JSON.stringify({
      events: paginatedEvents || [],
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
