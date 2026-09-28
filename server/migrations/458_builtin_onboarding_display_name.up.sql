-- 457 intentionally retained message history. Repair only the two pristine,
-- product-authored onboarding rows that have never been used by a real turn.
-- Names inside workspace/profile data are owner content and must survive.
DO $migration$
DECLARE
    candidate RECORD;
    greeting RECORD;
    legacy_suffix TEXT;
    current_suffix TEXT;
    current_opening TEXT;
    quoted_opening TEXT;
    current_quote TEXT;
    quote_position INTEGER;
    en_prefix CONSTANT TEXT := 'Hi — welcome to ';
    zh_prefix CONSTANT TEXT := '你好，欢迎来到 ';
    en_suffix CONSTANT TEXT := $en$. Multica is a workspace where you and AI agents coordinate real work through issues.

I'm Mika, your Chief of Staff here. I shape what needs doing, bring in the right agent for it, and stay your starting point for anything.

Here's how we begin: you name a goal, I turn it into an issue and start it with the right agent — and you watch it run.

Pick one below, or just tell me what you want to get done right now.$en$;
    zh_suffix CONSTANT TEXT := $zh$。Multica 是一个人和 AI 智能体通过任务一起把事情做完的工作区。

我是 Mika，这里的 Chief of Staff。我负责把事情理清楚、找到合适的智能体接手，也是你随时可以开口的第一站。

接下来是这样：你说一个目标，我把它变成一个任务，交给合适的智能体开始跑，你能看着它推进。

从下面选一个开始，或者直接告诉我你现在想做成什么。$zh$;
BEGIN
    FOR candidate IN
        SELECT session.id
        FROM chat_session AS session
        JOIN agent ON agent.id = session.agent_id AND agent.workspace_id = session.workspace_id
        WHERE agent.system_key = 'mika' AND agent.name = '小阿孚'
          AND EXISTS (
              SELECT 1 FROM chat_message
              WHERE chat_session_id = session.id AND message_kind = 'onboarding_opening'
          )
        ORDER BY session.id
    LOOP
        -- Match the first-send lock. Eligibility must be a SEPARATE statement
        -- after waiting: the candidate cursor can predate a committed first turn.
        PERFORM id FROM chat_session WHERE id = candidate.id FOR UPDATE;

        SELECT opening.id AS opening_id, opening.content AS opening_content,
               kickoff.id AS kickoff_id, kickoff.content AS kickoff_content
        INTO greeting
        FROM chat_session AS session
        JOIN agent ON agent.id = session.agent_id AND agent.workspace_id = session.workspace_id
        JOIN chat_message AS opening ON opening.chat_session_id = session.id
        JOIN chat_message AS kickoff ON kickoff.chat_session_id = session.id
        WHERE session.id = candidate.id
          AND agent.system_key = 'mika' AND agent.name = '小阿孚'
          AND opening.role = 'assistant' AND opening.message_kind = 'onboarding_opening'
          AND kickoff.role = 'user' AND kickoff.message_kind = 'onboarding_kickoff'
          AND opening.task_id IS NULL AND kickoff.task_id IS NULL
          AND (SELECT count(*) FROM chat_message WHERE chat_session_id = session.id) = 2
          AND NOT EXISTS (SELECT 1 FROM agent_task_queue WHERE chat_session_id = session.id);

        IF NOT FOUND THEN
            CONTINUE;
        END IF;

        -- The whole historical template must match, except for the old workspace
        -- display name. Do not compare today's workspace name: it may have changed.
        IF left(greeting.opening_content, length(en_prefix)) = en_prefix
           AND right(greeting.opening_content, length(en_suffix)) = en_suffix
           AND length(greeting.opening_content) >= length(en_prefix) + length(en_suffix) THEN
            legacy_suffix := en_suffix;
            current_suffix := replace(en_suffix, 'I''m Mika,', 'I''m 小阿孚,');
        ELSIF left(greeting.opening_content, length(zh_prefix)) = zh_prefix
           AND right(greeting.opening_content, length(zh_suffix)) = zh_suffix
           AND length(greeting.opening_content) >= length(zh_prefix) + length(zh_suffix) THEN
            legacy_suffix := zh_suffix;
            current_suffix := replace(zh_suffix, '我是 Mika，', '我是 小阿孚，');
        ELSE
            CONTINUE;
        END IF;

        current_opening := left(greeting.opening_content, length(greeting.opening_content) - length(legacy_suffix)) || current_suffix;
        quoted_opening := E'<opening-already-sent>\n' || greeting.opening_content || E'\n</opening-already-sent>';
        current_quote := E'<opening-already-sent>\n' || current_opening || E'\n</opening-already-sent>';
        quote_position := strpos(greeting.kickoff_content, quoted_opening);
        IF quote_position = 0 OR strpos(substring(greeting.kickoff_content FROM quote_position + length(quoted_opening)), quoted_opening) > 0 THEN
            CONTINUE;
        END IF;

        -- One statement and one transaction keep visible text and model context
        -- consistent, retaining every message ID, timestamp and ordering field.
        UPDATE chat_message
        SET content = CASE id
            WHEN greeting.opening_id THEN current_opening
            ELSE overlay(greeting.kickoff_content PLACING current_quote FROM quote_position FOR length(quoted_opening))
        END
        WHERE id IN (greeting.opening_id, greeting.kickoff_id);
    END LOOP;
END
$migration$;
