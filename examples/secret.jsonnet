local k = import "ksonnet.beta.1/k.libsonnet";
local util = import "ksonnet.beta.1/util.libsonnet";

local secret = k.core.v1.secret;

local namespace = "default";

// Here is my super-secret data
local data = {foo: std.base64("sekret")};

secret.default("mysecret", namespace) +
  secret.data(data)
