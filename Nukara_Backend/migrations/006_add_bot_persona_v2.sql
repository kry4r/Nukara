ALTER TABLE bots ADD COLUMN IF NOT EXISTS relationship TEXT NOT NULL DEFAULT '';
ALTER TABLE bots ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT '';
ALTER TABLE bots ADD COLUMN IF NOT EXISTS self_cognition JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE bots ADD COLUMN IF NOT EXISTS persona_prompt TEXT NOT NULL DEFAULT '';
ALTER TABLE bots ADD COLUMN IF NOT EXISTS persona_version INT NOT NULL DEFAULT 1;
ALTER TABLE bots ADD COLUMN IF NOT EXISTS identity TEXT NOT NULL DEFAULT '';
ALTER TABLE bots ADD COLUMN IF NOT EXISTS personality JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE bots ADD COLUMN IF NOT EXISTS expression_style TEXT NOT NULL DEFAULT '';
ALTER TABLE bots ADD COLUMN IF NOT EXISTS life_context TEXT NOT NULL DEFAULT '';
ALTER TABLE bots ADD COLUMN IF NOT EXISTS taboos_and_preferences TEXT NOT NULL DEFAULT '';

UPDATE bots
SET identity = COALESCE(NULLIF(identity, ''), NULLIF(summary, ''), relationship),
    personality = CASE
        WHEN personality = '[]'::jsonb OR personality IS NULL THEN COALESCE(traits, '[]'::jsonb)
        ELSE personality
    END,
    expression_style = COALESCE(NULLIF(expression_style, ''), speaking_style),
    life_context = COALESCE(NULLIF(life_context, ''), NULLIF(background, ''), role),
    taboos_and_preferences = COALESCE(
        NULLIF(taboos_and_preferences, ''),
        NULLIF(array_to_string(ARRAY(SELECT jsonb_array_elements_text(self_cognition)), '；'), ''),
        ''
    )
WHERE identity = ''
   OR personality = '[]'::jsonb
   OR expression_style = ''
   OR life_context = ''
   OR taboos_and_preferences = '';
