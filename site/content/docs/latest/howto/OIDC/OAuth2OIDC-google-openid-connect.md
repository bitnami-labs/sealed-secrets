# Configuring Google OIDC as an OIDC provider

This document explains how to configure Google OIDC as an OIDC provider (check general information and pre-requisites for [using an OAuth2/OIDC Provider with Kubeapps](../../tutorials/using-an-OIDC-provider.md)).

In the case of Google we can use an OAuth 2.0 client ID. You can find more information [here](https://developers.google.com/identity/protocols/OpenIDConnect). In particular we will use a "Web Application".

- **Client-ID**: `<abc>.apps.googleusercontent.com`.
- **Client-Secret**: Secret for the Web application.
- **OIDC Issuer URL**: <https://accounts.google.com>.
