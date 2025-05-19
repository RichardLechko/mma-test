interface GTagEvent {
  event_category?: string;
  event_label?: string;
  value?: number;
  [key: string]: any;
}

declare global {
  interface Window {
    gtag: (command: string, action: string, params?: GTagEvent | string) => void;
    dataLayer: any[];
  }
}

const isGtagAvailable = (): boolean => {
  return typeof window !== 'undefined' && window.gtag != null;
};

export const trackEvent = (
  eventName: string,
  params?: GTagEvent
): void => {
  if (isGtagAvailable()) {
    window.gtag('event', eventName, params);
  }
};

export const trackFighterView = (fighterId: string, fighterName: string): void => {
  trackEvent('view_fighter', {
    event_category: 'Fighter',
    event_label: fighterName,
    fighter_id: fighterId
  });
};

export const trackEventView = (eventId: string, eventName: string): void => {
  trackEvent('view_event', {
    event_category: 'Event',
    event_label: eventName,
    event_id: eventId
  });
};

export const trackSearch = (searchTerm: string, resultsCount: number): void => {
  trackEvent('search', {
    event_category: 'Engagement',
    event_label: searchTerm,
    results_count: resultsCount
  });
};

export const trackFilterUse = (filterType: string, filterValue: string): void => {
  trackEvent('use_filter', {
    event_category: 'Engagement',
    event_label: `${filterType}: ${filterValue}`,
    filter_type: filterType,
    filter_value: filterValue
  });
};