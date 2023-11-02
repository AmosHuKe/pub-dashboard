# pub-dashboard

## Setup 💻

1.Add comments to the place where you want to update in the markdown file.

```
<!-- md:PubDashboard start --> 
<!-- md:PubDashboard end -->
```

2.Enable read/write permissions

> Settings -> Actions -> General -> Workflow permissions -> Read and write permissions 

3.Edit the settings in `.github/workflows/pub-dashboard.yml`

```yaml
...

jobs:
  pub-dashboard-job:
    runs-on: ubuntu-latest
    name: pub-dashboard
    steps:
      - name: run pub-dashboard
        uses: AmosHuKe/pub-dashboard@main
        with:
          filename: "test.md"
          package_list: "flutter_tilt,bb,cc"
          sort_field: "published"
          sort_mode: "asc"

...
```

| Setting | Default | Value | Description |  
|---------|---------|-------|-------------|
| filename | README.md | | Markdown file <br/> e.g. "README.md" "test/test.md" |
| package_list <sup>`required`</sup> | | | Package name (`,` split) <br/> e.g. "aa,bb,cc" |
| sort_field | name | name, published | Sort field |
| sort_mode | asc | asc, desc | Sort mode |

## Tips 💡

- ⁉️: Package not found
- The `Github link` is resolved by the `Homepage`, `Repository`, `IssueTracker` of `pub.dev`

## License 📄

Open sourced under the [Apache-2.0](LICENSE).

© AmosHuKe