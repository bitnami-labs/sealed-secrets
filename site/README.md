# Website for [Kubeapps](https://kubeapps.com/)

## Deployment

The website will be deployed to production each time a new commit gets merged into the main branch. It is deployed using Netlify. The [netlify.toml](./netlify.toml) file holds the configuration.

## Requirements

This site uses [Hugo](https://github.com/gohugoio/hugo) for rendering. It is recommended you run `hugo` locally to validate your changes render properly.

### Local Hugo Rendering

Hugo is available for many platforms. It can be installed using:

- Linux: Most native package managers
- macOS: `brew install hugo`
- Windows: `choco install hugo-extended -confirm`

Once installed, you may run the following from the `/site` directory to access a rendered view of the documentation:

```bash
cd site
hugo server --disableFastRender
```

Access the site at [http://localhost:1313](http://localhost:1313). Press `Ctrl-C` when done viewing.

The [site/content/docs/latest](./content/docs/latest) directory holds the project documentation whereas [site/themes/template/static../img/docs](./themes/template/static../img/docs) contains the images used in the documentation. Note they have to be under that folder to be properly served.

## Check writing

In order to validate and ensure a proper style of writing, it is recommended to run the [vale validator](https://vale.sh/docs/vale-cli/installation/) with a set of [style rules](https://github.com/errata-ai/styles). The rules are present in the project codebase in the directory. Some of them have been slightly modified to fit our project needs.

To run the validator, install the `vale` binary on your machine from the [vale releases website](https://github.com/errata-ai/vale/releases) and run:

```bash
cd site
vale --config ./vale.ini ./content/
```

## Check links

In addition to the style check, it is also recommended to run a link checker to detect broken links and other issues.
First, render the website locally and then run [check-html-links](https://www.npmjs.com/package/check-html-links) in the `site/public` directory.

```bash
cd site
hugo
npx check-html-links ./public/
```

## Check formatting

Also, another tool for checking the markdown syntax are [markdownlint-cli](https://github.com/igorshubovych/markdownlint-cli) and [prettier](https://github.com/prettier/prettier). To use them, run:

```bash
cd site
npx markdownlint-cli .\content\docs\latest\ --disable MD013 MD033 # add --fix to also solve the issues
npx prettier --write .\content\docs\latest\
```

## Check accessibility

In order to validate the accessibility conformance, it is recommended to run the [pa11y validator](https://github.com/pa11y/pa11y).
First, serve the website locally and then run `pa11y` in the `http://localhost:1313/` address.

```bash
cd site
hugo server --disableFastRender
npx pa11y http://localhost:1313/ -i "WCAG2AA.Principle1.Guideline1_4.1_4_3.G18.Fail" # ignoring this as colors are set by the corporate template
```
