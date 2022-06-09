# Custom Form Component Support

This is an extension to the [basic form support](./basic-form-support.md#basic-form-support)

## Possible use cases

- Custom UI component that are not yet supported by the basic components (e.g: radio selectors)
- Consuming third party APIs for component values and validation

## Step-by-step integration process

1. First you will need a react component to render instead of the default Kubeapps form components. You're React components must be compiled into a JS file so that they can be interpreted by the browser since they cannot natively parse `.jsx` or `.tsx` files. You can compile `jsx` or `tsx` into js with tools like webpack, create-react-app, babel, etc. If you just want to try this feature out and you don't have a component yet we provide some test files you can try (Do not try to load the `jsx` file since browsers cannot parse it! We simply include it so that you can see the pre-compiled version of the `.js` files).
2. The easiest way to add inject the file in is via the command line. You can do it via the following command:

   ```bash
   helm install  bitnami/kubeapps --set-file dashboard.customComponents=*path to file* <other_flags>
   ```

   Note: The file can be located anywhere on your file system or even a remote source!
   Alternatively we provide remote loading by setting the `remoteComponentsUrl` value to the URL that is serving your bundle. If this is not set, the configmap will be the default loader.

3. Once the deployment is complete you will need a values json that will signal to Kubeapps that we want to render a custom component and not one fo the provided ones. To do that you will need a `values.json.schema` that has a `customComponent` key, more info [here](./custom-form-component-support.md#render-a-custom-component).

## Render a custom component

Signaling to the Kubeapps dashboard that you want to render a custom component is pretty straight forward. Simply add a `customComponent` field to any form parameter defined in the `values.json.schema` and that will tell the react application to fetch the component from the custom js bundle. An example parameter could look like:

```json
    "databaseType": {
      "type": "string",
      "form": true,
      "enum": ["mariadb", "postgresql"],
      "title": "Database Type",
      "description": "Allowed values: \"mariadb\" and \"postgresql\"",
      "customComponent": {
          "type": "radio",
          "className": "primary-radio"
      }
    }
```

Note: The `customComponent` field **MUST BE AN OBJECT**. This design decision was made so that developers can pass extra values/properties into their custom components should they require them.

## Updating Helm values with custom components

Custom form components would be useless without the ability to interact with the YAML state. To do this your custom components should be set up to receive 2 props: `handleBasicFormParamChange` and `param`. `param` is the current json object this is being rendered (denoted by the `customComponent` field) and `handleBasicFormParamChange` which is a function that updates the YAML file. An example of how you use this function can be found in any of the BasicDeploymentForm components such as the [SliderParam](https://github.com/vmware-tanzu/kubeapps/blob/main/dashboard/src/components/DeploymentFormBody/BasicDeploymentForm/SliderParam.tsx#L47-L53).

```javascript
  const handleParamChange = (newValue: number) => {
    handleBasicFormParamChange(param)({
      currentTarget: {
        value: newValue,
      },
    } as React.FormEvent<HTMLInputElement>);
  };
```

## Tips

To help you get started we provide some examples that you can try [here](https://github.com/vmware-tanzu/kubeapps/tree/main/site/content/docs/latest/reference/examples). The three files should give you a good idea about how to start developing and building your own custom components. `CustomComponent.jsx` is a super simple react component that takes the `handleBasicFormParamChange` and `param` props and renders a button that changes the value to 'test'. `CustomComponent.js` is the JavaScript variant of `CustomComponent.jsx` and `CustomComponent.min.js` is a minified js bundle created using [remote-component-starter](https://github.com/Paciolan/remote-component-starter), which is specifically made to help build components that you want to load remotely with the [remote-component](https://github.com/Paciolan/remote-component) tool used by Kubeapps.
