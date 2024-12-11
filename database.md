# CORE ENTITIES:

## 1. Promotion (Top-Level Entity)
### Core Promotion Info:
- ID, name, country, website
- Organization-specific rules and ranking systems
- Broadcasting partnerships and platforms
- Social media and official channels
- Created/Updated timestamps for tracking changes

## 2. Fighters
### Basic Info: 
- ID, full name, nickname, nationality, date of birth, gender
- Physical attributes: stance, reach (cm), height (cm)
- Weight class(es) history
- Standardized measurements in metric

### Career Stats:
- Record (wins-losses-draws-no contests)
- Method of victory breakdown (KO/TKO, submissions, decisions)
  
### Status: 
- Promotion-specific rankings
- Last fight date
  
### Social Media: 
- Official social handles, website
- Verified status
  
### Media: 
- Profile photo URL, banner photo URL
- Walkout/highlight footage rights

## 3. Events
### Basic Info: 
- ID, event name
- Promotion reference (foreign key)
- Date and time (with timezone)
- Venue details
  
### Location: 
- City, state/province, country
- Venue capacity
- Geographic coordinates for location-based queries
  
### Status: 
- Event lifecycle (announced/scheduled/completed/canceled)
- Change history tracking
  
### Broadcast: 
- Primary broadcaster
- Multiple streaming platform support
- Region-specific broadcast rights
  
### Event Type: 
- Format (PPV/Fight Night/Tournament/etc.)
- Special event designation
  
### Media: 
- Poster URL, banner URL
- Broadcasting rights info

## 4. Fight Cards
- Optional relationship with events (can be single card)
- Card type designation (main/prelim/early)
- Independent broadcast information
- Start and end times (with timezone)
- Order within event

## 5. Fights (Bout)
- Fight ID
- Weight class (enumerated type)
- Gender-specific weight class handling
- Scheduled rounds and time limits
- Bout order on card
- Status tracking
- Comprehensive result data
- Performance bonuses
- Created/Updated timestamps

## 6. Users
### Basic Info:
- ID, email, name
- Timezone handling
- Language preferences
  
### Authentication:
- Multiple auth provider support
- Session management
- Security logs
  
### Settings:
- Granular notification controls
- Display preferences
- Data privacy settings
  
### Account Status:
- Active/Inactive flag
- Subscription management
- Account history

## 7. User Preferences
### Display preferences: 
- Theme settings (dark/light/system)
- Weight class filtering
- Promotion filtering
- Language/region settings

### Notification settings: 
- Per-event notification rules
- Channel preferences (email/push/SMS)
- Frequency controls
- Calendar integration preferences

### Following:
- Favorite fighters (with notification rules)
- Followed promotions
- Custom fight/event reminders

## 8. Watchlist
- User reference
- Event/Fight reference
- Watch status (watching/maybe/not watching)
- Custom reminder settings
- Personal notes
- Sharing preferences

## 9. Rankings (New Entity)
- Fighter reference
- Promotion reference
- Weight class
- Current rank
- Ranking history
- Last updated timestamp

# RELATIONSHIPS:

## 1. Event to Fight Cards: One-to-Many
- Events can have one OR more fight cards
- Flexible structure for different promotion formats
- Maintains event integrity

## 2. Fight Cards to Fights: One-to-Many
- Ordered fight listing
- Position tracking
- Broadcast segment alignment

## 3. Fights to Fighters: Two-to-One
- Explicit red corner and blue corner assignment
- One fight per event per fighter constraint
- Historical bout tracking

## 4. Users to Watchlist: One-to-Many
- Enhanced tracking capabilities
- Personalized notes and reminders
- Sharing options

## 5. Fighters to Users (Favorites): Many-to-Many
- Notification preferences per fighter
- Follow history tracking
- Interaction logging

## 6. Promotion to Events: One-to-Many
- Organizational hierarchy
- Promotion-specific event rules
- Broadcasting rights management

# ADDITIONAL TRACKING:

## 1. Fight History
- Complete status change history
- Card position changes
- Weight changes and missing weight incidents
- Result modifications

## 2. Event Updates
- Comprehensive change tracking
- Venue/location modifications
- Broadcast updates
- Card structure changes

## 3. Fighter Status Changes
- Weight class history
- Ranking changes per promotion
- Injury status tracking
- Contract status (optional)

## 4. Broadcasting Rights (New)
- Region-specific availability
- Platform rights
- Blackout rules
- Replay availability

# INDEXES AND SEARCH CONSIDERATIONS:

## 1. Primary Search Fields
- Fighter names (including nicknames and variations)
- Event details (name, location, date)
- Promotion-specific searches
- Weight class and gender combinations

## 2. Common Query Patterns
- Upcoming events (location and time-based)
- Fighter schedules and history
- Promotion-specific event listings
- Weight class-specific queries
- Recent results and rankings

## 3. Performance Considerations
- Multi-language search support
- Geospatial indexing
- Temporal data optimization
- Composite indexes for common filters
- Caching strategies for frequent queries

This schema is designed to be:
1. Promotion-agnostic but promotion-aware
2. Globally accessible and timezone-conscious
3. Highly scalable for future feature additions
4. Optimized for common user interactions
5. Historically accurate with full audit capabilities
