# Custom Fields

lazyjira can display Jira custom fields in the info panel.

## Setup

Add custom fields to your `config.yml`.

```yaml
customFields:
  - id: "customfield_10015"
    name: "Story Points"
  - id: "customfield_10020"
    name: "Team"
```

## Field properties

| Property | Required | Description |
|----------|----------|-------------|
| `id` | yes | Jira custom field ID, for example `customfield_10015` |
| `name` | yes | Display name shown in the info panel |

## Finding field IDs

You can find custom field IDs in Jira by going to your project settings or by using the Jira REST API.

```
GET /rest/api/3/field
```

Look for fields where `custom` is true. The `id` value is what you need.
