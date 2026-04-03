# Fields

lazyjira shows issue fields in the info panel. Without config it shows defaults: status, priority, assignee, reporter, issuetype, sprint (plus labels and components when set).

To customize which fields appear, add a `fields:` section to `config.yml`. This replaces the defaults entirely. Order matters.

## Basic setup

```yaml
fields:
  - id: status
  - id: priority
  - id: assignee
  - id: reporter
  - id: issuetype
  - id: sprint
  - id: labels
  - id: components
  - id: "customfield_10015"
    name: "Story Points"
  - id: "customfield_10020"
    name: "Team"
```

## Builtin fields

These IDs are recognized as builtin and get special rendering (colors, styling).

| ID | Default name |
|----|-------------|
| `status` | Status |
| `priority` | Priority |
| `assignee` | Assignee |
| `reporter` | Reporter |
| `issuetype` | Type |
| `sprint` | Sprint |
| `labels` | Labels |
| `components` | Components |

You can override the display name with `name`.

## Custom field properties

| Property | Required | Description |
|----------|----------|-------------|
| `id` | yes | Jira field ID, for example `customfield_10015` |
| `name` | no | Display name. Falls back to the raw ID if omitted |
| `type` | no | Force field type: `select`, `multiselect`, `user`, `text`, `textarea`. Default: auto-detect from value |
| `multiline` | no | Open external editor instead of inline input for text fields. Default: false |

## Editing

Press `e` on any field in the info panel to edit it.

- **select** fields show a picker with allowed values from Jira
- **multiselect** fields show a checklist
- **user/person** fields show a user picker
- **text** fields show an inline input
- **textarea** or fields with `multiline: true` open your `$EDITOR`

Fields with explicit `type: text` or `type: textarea` skip the CreateMeta API call and go straight to the input.

If a field is not available for editing on the issue type, an error message is shown.

## Type auto-detection

When `type` is not set, lazyjira detects the field type from the Jira value:

- Object with `displayName` key: person
- Object with `value` or `name` key: select
- Array: multiselect
- Everything else: text

## Finding field IDs

You can find custom field IDs in Jira via the REST API.

```
GET /rest/api/3/field
```

Look for fields where `custom` is true. The `id` value is what you need.

## Migration from customFields

The old `customFields:` config key still works but is deprecated. If you have `customFields:` and no `fields:`, the values are automatically migrated. Rename `customFields` to `fields` in your config when convenient.
