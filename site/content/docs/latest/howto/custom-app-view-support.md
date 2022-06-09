# Custom App View Support

In addition to our [custom form component support](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/howto/custom-form-component-support.md) we now support the ability for developers to inject custom app views for specific deployments.

## Step-by-step integration process

1. First you will need a react component to render instead of the default Kubeapps Application View. You're React components must be compiled into a JS file so that they can be interpreted by the browser since they cannot natively parse `.jsx` or `.tsx` files. You can compile `jsx` or `tsx` into js with tools like webpack, create-react-app, babel, etc. If you just want to try this feature out and you don't have a component yet we provide some test files you can try (Do not try to load the `jsx` file since browsers cannot parse it! We simply include it so that you can see the pre-compiled version of the `.js` files).

1. Next you will need to define which applications you would like to render the custom view for. To do this we simply set `.Values.dashboard.customAppViews` to any application of your choice. For example, if you wanted to load a custom view for the [bitnami apache helm chart](https://github.com/bitnami/charts/tree/master/bitnami/apache) you can set the value as such:

   ```yaml
   customAppViews:
     - plugin: helm.packages
       name: apache
       repository: bitnami
   ```

   This will tell the frontend to load the custom bundle for the apache helm chart in the bitnami repo.

1. And just like the custom form components the bundle can be added via the command line:

   ```bash
   helm install  bitnami/kubeapps --set-file dashboard.customComponents=*path to file* <other_flags>
   ```

   Note: The file can be located anywhere on your file system or even a remote source!
   Or you can set the `.Values.dashboard.remoteComponentsUrl` to a bundle served by a remote server.

## Example Code

In an effort to make getting antiquated with the feature easier we provide some demo code for you to play around with and explore the props that the dashboard supplies. The examples can be found in the [developer documentation examples](https://github.com/vmware-tanzu/kubeapps/tree/main/site/content/docs/latest/reference/examples). [CustomAppView.jsx](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/reference/examples/CustomAppView.jsx) is a demo Application View that will get you familiar w/ the props and handlers that we pass as props. The props are displayed in plain text and the buttons are wired up to use the handlers that mirror the buttons in the normal Application View. We also provide a complied bundle version [CustomAppView.min.js](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/reference/examples/CustomAppView.min.js) which you can load into the configmap and render.
