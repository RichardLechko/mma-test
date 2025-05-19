import { createClient } from '@supabase/supabase-js';

const supabaseUrl = "https://rkwgvnkasghxctojvdhm.supabase.co";
const supabaseAnonKey = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InJrd2d2bmthc2doeGN0b2p2ZGhtIiwicm9sZSI6ImFub24iLCJpYXQiOjE3MzQ3NjM3OTIsImV4cCI6MjA1MDMzOTc5Mn0.FCjn9Z2WPKHSrt1PGvL93Gb-RMwnz68wTrOuo89vWkw";
const supabase = createClient(supabaseUrl, supabaseAnonKey);

export { supabase as s };
