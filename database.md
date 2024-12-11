# CORE ENTITIES:

## 1. Promotion (Top-Level Entity)

### Core Promotion Info:
- name, country, website

- Critical for separating different organizations' rules and structures
- Allows tracking of promotion-specific rankings
- Enables users to follow specific promotions

## 1. Fighters

### Basic Info: 
- ID, full name, nickname, nationality, date of birth, weight class(es), stance, reach, height, gender
  
### Career Stats:
- record (wins-losses-draws-no contests), knockouts, submissions
  
### Status: 
- active/retired (boolean flag), ranking (if any), last fight date
  
### Social Media: 
- official social handles, website
  
### Media: 
- profile photo URL, banner photo URL

## 2. Events

### Basic Info: 
- ID, event name, promotion (UFC, Bellator, etc.), date and time, venue
  
### Location: 
- city, state/province, country
  
### Status: 
- announced/scheduled/completed/canceled
  
### Broadcast: 
- main broadcaster, streaming platforms
  
### Event Type: 
- PPV/Fight Night/etc.
  
### Media: 
- poster URL, banner URL

## 3. Fight Cards
- Main Card / Prelims designation
- Start time for each card section
- Broadcast information specific to card section

## 4. Fights (Bout)
- Fight ID
- Two fighters (references to Fighter table)
- Weight class for this specific bout
- Number of rounds scheduled
- Bout order on card
- Status (scheduled/completed/canceled)
- Result (if completed): winner, method, time, round
- Fight bonus awards (Fight of Night, Performance bonus etc.)

## 5. Users

### Basic Info:
- ID, email, name, timezone
  
### Authentication:
- auth provider details, last login
  
### Settings:
- notification preferences, display preferences
  
### Account Status:
- active/inactive, subscription tier if applicable

## 6. User Preferences

### Display preferences: 
- dark/light mode, preferred weight classes

### Notification settings: 
- email, push, SMS preferences
- Calendar sync preferences
- Favorite fighters list
- Followed promotions list

## 7. Watchlist
- User ID (reference to Users)
- Fight/Event ID
- Reminder settings
- Notes
- Status (watching/maybe/not watching)

# RELATIONSHIPS:

## 1. Event to Fight Cards: One-to-Many
- Each event has one or more fight cards (main card, prelims)
- Fight cards belong to exactly one event

## 2. Fight Cards to Fights: One-to-Many
- Each fight card contains multiple fights
- Each fight belongs to one fight card

## 3. Fights to Fighters: Many-to-Two
- Each fight has exactly two fighters
- Fighters can have many fights

## 4. Users to Watchlist: One-to-Many
- Users can watch many events/fights
- Each watchlist entry belongs to one user

## 5. Fighters to Users (Favorites): Many-to-Many
- Users can favorite multiple fighters
- Fighters can be favorited by multiple users

# ADDITIONAL TRACKING:

## 1. Fight History
- Tracks changes to fight status
- Records updates to fight card positioning
- Maintains history of weight changes

## 2. Event Updates
- Tracks changes to event details
- Records venue changes
- Maintains broadcast information updates

## 3. Fighter Status Changes
- Records weight class changes
- Tracks ranking updates
- Maintains injury status

# INDEXES AND SEARCH CONSIDERATIONS:

## 1. Primary Search Fields
- Fighter names (including nicknames)
- Event names and locations
- Dates (for events and fights)
- Weight classes

## 2. Common Query Patterns
- Upcoming events by date
- Fighter's upcoming fights
- Events by location
- Fights by weight class
- Recent results

## 3. Performance Considerations
- Full-text search on fighter names and event titles
- Date-based indexing for quick event lookup
- Geographic indexing for location-based queries
- Composite indexes for common filter combinations
