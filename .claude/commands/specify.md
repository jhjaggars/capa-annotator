# Specify - Start Requirements Gathering

Begin gathering requirements for: $ARGUMENTS

## Full Workflow:

### Phase 1: Initial Setup & Codebase Analysis
1. Extract slug from $ARGUMENTS (e.g., "add user profile" → "user-profile")
2. Read configuration from .spec/config.json (if available) to get question counts:
   - discovery_questions: number of discovery phase questions (default: 5)
   - expert_questions: number of expert phase questions (default: 5)
3. Use specware tool to create feature directory:
   ```
   $ specware feature new-requirements [slug]
   ```
4. This creates:
   - .spec/###-[slug]/ directory with sequential numbering
   - q-a-requirements.md file from template
   - requirements.md file from template
   - .spec-status file tracking progress
   - .spec/current.json file marking this feature as currently active
5. Use repository analysis tools (if available) to understand overall structure:
   - **Option A - RepoPrompt:** Use mcp__RepoPrompt__get_file_tree for text-based file tree analysis
   - **Option B - Serena:** Use mcp__serena__activate_project and mcp__serena__get_project_structure for semantic analysis
   - **Alternative:** Use mcp__serena__analyze_project for comprehensive project overview
   - Get high-level architecture overview
   - Identify main components and services
   - Understand technology stack
   - Note patterns and conventions

### Phase 2: Context Discovery Questions
6. Generate the [discovery_questions] most important yes/no questions to understand the problem space (read from .spec/config.json, default 5):
   - Questions informed by codebase structure
   - Questions about user interactions and workflows
   - Questions about similar features users currently use
   - Questions about data/content being worked with
   - Questions about external integrations or third-party services
   - Questions about performance or scale expectations
   - Write all questions to q&a-requirements.md with smart defaults under "## Discovery Questions" section
   - Begin asking questions one at a time proposing the question with a smart default option
   - Only after all questions are asked, record answers in q&a-requirements.md under "## Discovery Answers" section

### Phase 3: Targeted Context Gathering (Autonomous)
7. After all discovery questions answered:
   - **Search for specific code patterns:**
     - **RepoPrompt:** Use mcp__RepoPrompt__search for text-based file search
     - **Serena:** Use mcp__serena__find_symbol for semantic code search, mcp__serena__find_references for usage patterns
   - **Read relevant code files:**
     - **RepoPrompt:** Use mcp__RepoPrompt__set_selection and read_selected_files for batch reading
     - **Serena:** Use mcp__serena__get_file_content for targeted file reading
   - **Discover similar patterns:**
     - **Serena:** Use mcp__serena__find_similar_code for pattern discovery
     - **Serena:** Use mcp__serena__list_functions and mcp__serena__list_classes for component inventory
   - Deep dive into similar features and patterns
   - Analyze specific implementation details
   - Use mcp__serena__analyze_dependencies (if available) to understand code relationships
   - Use WebSearch and or context7 for best practices or library documentation
   - Document findings in q&a-requirements.md under "## Context Findings" section including:
     - Specific files that need modification
     - Exact patterns to follow
     - Similar features analyzed in detail
     - Technical constraints and considerations
     - Integration points identified

### Phase 4: Expert Requirements Questions
8. Now ask questions like a senior developer who knows the codebase:
   - Write the top [expert_questions] most pressing unanswered detailed yes/no questions to q&a-requirements.md under "## Expert Questions" section (read from .spec/config.json, default 5)
   - Questions should be as if you were speaking to the product manager who knows nothing of the code
   - These questions are meant to to clarify expected system behavior now that you have a deep understanding of the code
   - Include smart defaults based on codebase patterns
   - Ask questions one at a time
   - Only after all questions are asked, record answers in q&a-requirements.md under "## Expert Answers" section

### Phase 5: Requirements Documentation
9. Generate comprehensive requirements spec in requirements.md:
   - Problem statement and solution overview
   - Functional requirements based on all answers
   - Technical requirements with specific file paths
   - Implementation hints and patterns to follow
   - Acceptance criteria
   - Assumptions for any unanswered questions

## Question Formats:

### Discovery Questions (Phase 2):
```
## Q1: Will users interact with this feature through a visual interface?
**Default if unknown:** Yes (most features have some UI component)

## Q2: Does this feature need to work on mobile devices?
**Default if unknown:** Yes (mobile-first is standard practice)

## Q3: Will this feature handle sensitive or private user data?
**Default if unknown:** Yes (better to be secure by default)

## Q4: Do users currently have a workaround for this problem?
**Default if unknown:** No (assuming this solves a new need)

## Q5: Will this feature need to work offline?
**Default if unknown:** No (most features require connectivity)
```

### Expert Questions (Phase 4):
```
## Q7: Should we extend the existing UserService at services/UserService.ts?
**Default if unknown:** Yes (maintains architectural consistency)

## Q8: Will this require new database migrations in db/migrations/?
**Default if unknown:** No (based on similar features not requiring schema changes)
```

## Important Rules:
- ONLY yes/no questions with smart defaults
- ONE question at a time
- Write ALL questions to file BEFORE asking any
- Stay focused on requirements (no implementation)
- Use actual file paths and component names in detail phase
- Document WHY each default makes sense
- Use tools available if recommended ones aren't installed or available
- **MCP Server Options:** RepoPrompt provides text-based search, Serena provides semantic/LSP-based analysis
- **Fallback:** Use standard Read, Grep, and Glob tools if no MCP servers available
- **DO NOT include time estimates, resource planning, or team size recommendations in requirements documentation**

## File Structure:
```
.spec/
  config.json            # Configuration for question counts and settings
  ###-feature-name/
    q&a-requirements.md  # All Q&A phases consolidated
    requirements.md      # Final requirements specification
    .spec-status         # Current phase tracking

.spec/
  current.json           # Tracks currently active feature
  config.json            # Configuration for question counts and settings
```

## Configuration (.spec/config.json):
```json
{
  "requirements": {
    "discovery_questions": 5,
    "expert_questions": 5
  }
}
```

## Phase Transitions:
- After each phase, announce: "Phase complete. Starting [next phase]..."
- Save all work before moving to next phase
- Update .spec-status file with current phase
- User can check progress by reading .spec/current.json and the q&a-requirements.md file
- The created feature becomes the "current" feature for subsequent agent operations