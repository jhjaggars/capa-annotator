# Spec-Driven Development

This directory contains specifications for features and initiatives developed using a spec-driven workflow.

## Getting Started

1. **Create a new feature specification:**
   ```bash
   specware feature new-requirements <short-name>
   ```

2. **Use Claude Code to gather requirements:**
   ```bash
   claude
   > /specify Add a new feature description here
   ```

3. **Add implementation planning:**
   ```bash
   specware feature new-implementation-plan <short-name>
   ```

## Directory Structure

Each feature gets its own numbered directory:

```
.spec/
├── 001-user-authentication/
│   ├── requirements.md
│   ├── q-a-requirements.md
│   ├── implementation-plan.md
│   ├── q-a-implementation-plan.md
│   └── .spec-status
├── 002-dashboard-widget/
│   └── ...
└── config.json
```

## Configuration

Edit `.spec/config.json` to customize the number of questions asked during requirements gathering:

```json
{
  "requirements": {
    "discovery_questions": 5,
    "expert_questions": 5
  }
}
```

## Templates

Customize templates for your project:

```bash
specware localize-templates
```

This creates `.spec/templates/` with customizable template files.

## Workflow

1. **Requirements Phase:** Use `/specify` command in Claude Code
2. **Implementation Phase:** Use the generated requirements and plan to implement
3. **Status Tracking:** Check `.spec-status` files for current phase

For more information, see the individual specification directories.