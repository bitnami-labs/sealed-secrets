{
  CustomResourceDefinition(group, version, kind): {
    local this = self,
    apiVersion: 'apiextensions.k8s.io/v1',
    kind: 'CustomResourceDefinition',
    metadata+: {
      name: this.spec.names.plural + '.' + this.spec.group,
    },
    spec: {
      scope: 'Namespaced',
      group: group,
      versions_:: {
        [version]: {
          served: true,
          storage: true,
        },
      },
      versions: $.mapToNamedList(self.versions_),
      names: {
        kind: kind,
        singular: $.toLower(self.kind),
        plural: self.singular + 's',
        listKind: self.kind + 'List',
      },
    },
  },
}
