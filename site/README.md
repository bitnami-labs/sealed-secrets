# Website for [Sealed Secrets](https://sealed-secrets.netlify.app/)

## Deployment

The website will be deployed to production each time a new commit gets merged into the main branch. It is deployed using Netlify.

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
