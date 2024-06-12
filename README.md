# pub-dashboard

Example: [Example.md](Example.md)

## Setup 💻

1.Add comments to the place where you want to update in the markdown file.

* Table

```
<!-- md:PubDashboard begin --><!-- md:PubDashboard end -->
```

* Package total

```
<!-- md:PubDashboard-total begin --><!-- md:PubDashboard-total end -->
```

2.Enable read/write permissions

(recommend) If you use a `Personal access token`:
> e.g. github_token: ${{ secrets.xxxxxx }} (ghp_xxxxx)  
> Create a token (enable repo permissions) https://github.com/settings/tokens/

If you use the current repo's token:
> e.g. github_token: ${{ secrets.GITHUB_TOKEN }}  
> https://docs.github.com/en/actions/security-guides/automatic-token-authentication  
> Current repo's settings: Settings -> Actions -> General -> Workflow permissions -> Read and write permissions 

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
          github_token: ${{ Personal access token }} or ${{ secrets.GITHUB_TOKEN }}
          github_repo: "https://github.com/AmosHuKe/pub-dashboard"
          filename: "Example.md"
          publisher_list: "fluttercandies.com"
          package_list: "extended_image,wechat_assets_picker,flutter_tilt"
          sort_field: "published"
          sort_mode: "asc"

...
```

| Setting | Default | Value | Description |  
|---------|---------|-------|-------------|
| github_token <sup>`required`</sup> | - | - | Github Token with repo permissions |
| github_repo <sup>`required`</sup> | - | - | Github repo to be manipulated |
| commit_message | docs(pub-dashboard): pub-dashboard has updated readme | - | Commit message |
| committer_username | github-actions[bot] | - | Committer username |
| committer_email | 41898282+github-actions[bot]@users.noreply.github.com | - | Committer email |
| filename | README.md | - | Markdown file <br/> e.g. "README.md" "test/test.md" |
| publisher_list | - | - | Publisher name (`,` split) <br/> e.g. "aa,bb,cc" |
| package_list | - | - | Package name (`,` split) <br/> e.g. "aa,bb,cc" |
| sort_field | name | name, published, pubLikes, githubStars | Sort field |
| sort_mode | asc | asc, desc | Sort mode |

## Tips 💡

- ⁉️: Package not found
- `publisher_list` and `package_list` are merged
- The `Github link` is resolved by the `Homepage`, `Repository`, `IssueTracker` of `pub.dev`

Thanks [Shields](https://github.com/badges/shields).

## License 📄

Open sourced under the [Apache-2.0](LICENSE).

© AmosHuKe