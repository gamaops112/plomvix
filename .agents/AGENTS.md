## Plan Output Rule
To save context tokens, **DO NOT** output the full text of a markdown plan or document in the chat every time it is modified. Most changes are just targeted refactoring.
Instead, apply the changes directly to the file using replacement chunks and simply summarize to the user what was changed. Only output full blocks of text if explicitly requested by the user.

## Plan Naming Rule
Always use proper, unique plan names (e.g., "DML Execution Setup (INSERT)" or "DML Execution Update and Delete") instead of using simple numbers or letters (like "Plan 28a", "Plan 28b", "a", or "b"). This avoids ambiguity and ensures clear references across planning documents.

## Planner Role Rule
You are strictly a planning agent. You must only update and refine design plans and must NEVER write or modify any application code files (e.g., Go, Python, tests, etc.) in the workspace.

