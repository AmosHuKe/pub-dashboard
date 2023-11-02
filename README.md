# pub-dashboard

Example: [Example.md](Example.md)

## Setup 💻

1.Add comments to the place where you want to update in the markdown file.

```
<!-- md:PubDashboard start --> 
<sub>Sort by published | Total 3</sub> 

| <sub>Package</sub> | <sub>Stars/Likes</sub> | <sub>Points/Popularity</sub> | <sub>Issues</sub> | <sub>Pull requests</sub> | 
|--------------------|------------------------|------------------------------|-------------------|--------------------------| 
| [assets_generator](https://pub.dev/packages/assets_generator) [![Pub package](https://img.shields.io/pub/v/assets_generator?label=)](https://pub.dev/packages/assets_generator) <br/> <sub>The flutter tool to generate assets‘s configs(yaml) and consts automatically for single project and multiple modules.</sub> <br/> <sub>Published: </sub> | [![GitHub stars](https://img.shields.io/github/stars/fluttercandies/assets_generator?style=social&logo=github&logoColor=1F2328&label=)](https://github.com/fluttercandies/assets_generator) <br/> [![Pub likes](https://img.shields.io/pub/likes/assets_generator?style=social&logo=flutter&logoColor=168AFD&label=)](https://pub.dev/packages/assets_generator) | [![Pub points](https://img.shields.io/pub/points/assets_generator?label=)](https://pub.dev/packages/assets_generator/score) <br/> [![popularity](https://img.shields.io/pub/popularity/assets_generator?label=)](https://pub.dev/packages/assets_generator/score) | [![GitHub issues](https://img.shields.io/github/issues/fluttercandies/assets_generator?label=)](https://github.com/fluttercandies/assets_generator/issues) | [![GitHub pull requests](https://img.shields.io/github/issues-pr/fluttercandies/assets_generator?label=)](https://github.com/fluttercandies/assets_generator/pulls) | 
| [waterfall_flow](https://pub.dev/packages/waterfall_flow) [![Pub package](https://img.shields.io/pub/v/waterfall_flow?label=)](https://pub.dev/packages/waterfall_flow) <br/> <sub>A Flutter grid view that build waterfall flow layout quickly.</sub> <br/> <sub>Published: </sub> | [![GitHub stars](https://img.shields.io/github/stars/fluttercandies/waterfall_flow?style=social&logo=github&logoColor=1F2328&label=)](https://github.com/fluttercandies/waterfall_flow) <br/> [![Pub likes](https://img.shields.io/pub/likes/waterfall_flow?style=social&logo=flutter&logoColor=168AFD&label=)](https://pub.dev/packages/waterfall_flow) | [![Pub points](https://img.shields.io/pub/points/waterfall_flow?label=)](https://pub.dev/packages/waterfall_flow/score) <br/> [![popularity](https://img.shields.io/pub/popularity/waterfall_flow?label=)](https://pub.dev/packages/waterfall_flow/score) | [![GitHub issues](https://img.shields.io/github/issues/fluttercandies/waterfall_flow?label=)](https://github.com/fluttercandies/waterfall_flow/issues) | [![GitHub pull requests](https://img.shields.io/github/issues-pr/fluttercandies/waterfall_flow?label=)](https://github.com/fluttercandies/waterfall_flow/pulls) | 
| [adaptation](https://pub.dev/packages/adaptation) [![Pub package](https://img.shields.io/pub/v/adaptation?label=)](https://pub.dev/packages/adaptation) <br/> <sub>A flutter adaptation library that is enlarged / reduced in proportion to the width of the design drawing.</sub> <br/> <sub>Published: </sub> | [![GitHub stars](https://img.shields.io/github/stars/fluttercandies/adaptation?style=social&logo=github&logoColor=1F2328&label=)](https://github.com/fluttercandies/adaptation) <br/> [![Pub likes](https://img.shields.io/pub/likes/adaptation?style=social&logo=flutter&logoColor=168AFD&label=)](https://pub.dev/packages/adaptation) | [![Pub points](https://img.shields.io/pub/points/adaptation?label=)](https://pub.dev/packages/adaptation/score) <br/> [![popularity](https://img.shields.io/pub/popularity/adaptation?label=)](https://pub.dev/packages/adaptation/score) | [![GitHub issues](https://img.shields.io/github/issues/fluttercandies/adaptation?label=)](https://github.com/fluttercandies/adaptation/issues) | [![GitHub pull requests](https://img.shields.io/github/issues-pr/fluttercandies/adaptation?label=)](https://github.com/fluttercandies/adaptation/pulls) | 
 
Updated on 2023-11-02T22:26:01+08:00 by [Action](https://github.com/AmosHuKe/pub-dashboard). 
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
          filename: "Example.md"
          package_list: "flutter_tilt,bb,cc"
          sort_field: "published"
          sort_mode: "asc"

...
```

| Setting | Default | Value | Description |  
|---------|---------|-------|-------------|
| filename | README.md | - | Markdown file <br/> e.g. "README.md" "test/test.md" |
| package_list <sup>`required`</sup> | - | - | Package name (`,` split) <br/> e.g. "aa,bb,cc" |
| sort_field | name | name, published | Sort field |
| sort_mode | asc | asc, desc | Sort mode |

## Tips 💡

- ⁉️: Package not found
- The `Github link` is resolved by the `Homepage`, `Repository`, `IssueTracker` of `pub.dev`

## License 📄

Open sourced under the [Apache-2.0](LICENSE).

© AmosHuKe