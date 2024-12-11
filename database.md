# CORE ENTITIES:

## 1. Fighters

### Basic Info: 
- ID, full name, nickname, nationality, date of birth, weight class(es), stance, reach, height
### Career Stats:
- record (wins-losses-draws-no contests), knockouts, submissions
### Status: 
- active/retired, ranking (if any), last fight date
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

### Fight ID
### Two fighters (references to Fighter table)
### Weight class for this specific bout
### Number of rounds scheduled
### Bout order on card
### Status (scheduled/completed/canceled)
Result (if completed): winner, method, time, round
Fight bonus awards (Fight of Night, Performance bonus etc.)


Users


Basic Info: ID, email, name, timezone
Authentication: auth provider details, last login
Settings: notification preferences, display preferences
Account Status: active/inactive, subscription tier if applicable


User Preferences


Display preferences: dark/light mode, preferred weight classes
Notification settings: email, push, SMS preferences
Calendar sync preferences
Favorite fighters list
Followed promotions list


Watchlist


User ID (reference to Users)
Fight/Event ID
Reminder settings
Notes
Status (watching/maybe/not watching)
