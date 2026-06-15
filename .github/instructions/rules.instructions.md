---
description: Describe when these instructions should be loaded by the agent based on task context
# applyTo: 'Describe when these instructions should be loaded by the agent based on task context' # when provided, instructions will automatically be added to the request context when the pattern matches an attached file
---

<!-- Tip: Use /create-instructions in chat to generate content with agent assistance -->

Provide project context and coding guidelines that AI should follow when generating code, answering questions, or reviewing changes.


Keep the setup minimal.
Do not create folders that are not needed right now.
Do not add WAL.
Do not add storage engine code.
Do not add query engine code.
Do not add API server code.
Do not add UI code.
Do not add external Go dependencies.
Do not add future placeholders unless explicitly required.
Complete only the task defined in this file.
Verify the project builds before marking the task complete.
Search Graphify before starting the task if Graphify is available in the local environment.
Update Graphify after completing the task if Graphify is available in the local environment.
If Graphify is not available, do not block the task; mention that Graphify was unavailable.