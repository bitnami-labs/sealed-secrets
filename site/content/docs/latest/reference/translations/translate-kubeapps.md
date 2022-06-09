# How to translate Kubeapps

> Important: this is a _work in progress_ feature. See the current progress at <https://github.com/vmware-tanzu/kubeapps/issues/2346>:

There are two ways to have Kubeapps with different messages: i) customizing the strings at runtime from the `values.yaml` file; ii) adding a new official translation into the Kubeapps project.

## Customizing Kubeapps literals in runtime

In case you want to use a custom translation (e.g., for adapting Kubeapps to your corporate needs), you only need to add json with the `"message-id": "translation"` you want to customize.

```yaml
customLocale: |-
  {
    "Kubeapps": "My dashboard",
    ...
  }
```

> Precompiled messages will result in a better overall application performance, instead of adding this json, you can follow the steps below.

## Adding a new translation to the Kubeapps project

The `/dashboard/lang` folder holds the current supported translations in Kubeapps. You can use one of these files, for instance, `en.json`, to generate the translation into your language.

Then, these json files are to be AST-compiled and served in the react app from the folder `/dashboard/src/locales`.

The process is as follows:

1. Select a base file in the `/dashboard/lang` folder. For instance, `en.json`.
2. Translate the strings into your language and save the document as `XX.json`, where `XX` is a two-letter [ISO 639-1 code](https://en.wikipedia.org/wiki/List_of_ISO_639-1_codes).
3. Run `yarn compile-lang` to generate the AST-compiled strings, this will improve the application's performance.
4. Add your language to the list of supported ones in the code.
5. Feel free to contribute with a new language and send a PR :)

> Developers should note that when adding a new literal, `yarn extract-lang` must be run.
