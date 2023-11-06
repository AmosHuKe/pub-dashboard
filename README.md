# pub-dashboard

Example: [Example.md](Example.md)

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
        uses: AmosHuKe/pub-dashboard@v0.1.0
        with:
          filename: "Example.md"
          publisherList: "fluttercandies.com,bb,cc"
          package_list: "flutter_tilt,bb,cc"
          sort_field: "published"
          sort_mode: "asc"

...
```

| Setting | Default | Value | Description |  
|---------|---------|-------|-------------|
| filename | README.md | - | Markdown file <br/> e.g. "README.md" "test/test.md" |
| publisher_list | - | - | Publisher name (`,` split) <br/> e.g. "aa,bb,cc" |
| package_list | - | - | Package name (`,` split) <br/> e.g. "aa,bb,cc" |
| sort_field | name | name, published | Sort field |
| sort_mode | asc | asc, desc | Sort mode |

## Tips 💡

- ⁉️: Package not found
- `publisher_list` and `package_list` are merged
- The `Github link` is resolved by the `Homepage`, `Repository`, `IssueTracker` of `pub.dev`

Thanks [Shields](https://github.com/badges/shields).

## License 📄

Open sourced under the [Apache-2.0](LICENSE).

© AmosHuKe