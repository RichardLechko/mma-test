# MMA-Scheduler
This MMA software provides a comprehensive platform for scheduling and tracking MMA events, including fighter details, match dates, and categories. It aggregates and displays real-time data through web scraping, enabling users to stay updated with upcoming fights and results.

# Frontend

- Framework: Astro
- Language: TypeScript
- Styling: SCSS
- UI Components: ShadCN
- Network Requests: Axios
- State Management: Astro's built-in state management
- Hosting: Vercel

# Backend

- Language: Go (1.21+)
- Web Framework: Fiber
- Web Scraping:
-- Colly
-- GoQuery


- Scheduling: Robfig/cron
- Metrics: Prometheus
- Logging: Zerolog
- Rate Limiting: Fiber middleware
- Circuit Breaker: Sony Gobreaker
- Hosting: AWS ECS

# Database & Caching

- Primary Database: Supabase (PostgreSQL)
- Caching: Redis
- Search: PostgreSQL Full-Text Search

# Authentication

- Provider: Supabase Auth
- Features:

Magic Link
OAuth Providers



Containerization & Orchestration

Containerization: Docker
CI/CD: GitHub Actions

Testing

E2E Testing: Cypress
Backend Testing: Go's built-in testing framework

Error Tracking

Method: GitHub Issues

Proxy Rotation

Custom Go Implementation: Inline proxy rotation service
