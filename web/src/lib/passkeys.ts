import { api } from "./api";

function handlePasskeyError(reason: unknown): never {
  if (reason instanceof DOMException) {
    switch (reason.name) {
      case "NotAllowedError":
        throw new Error("Passkey cancelled or timed out");
      case "AbortError":
        throw new Error("Passkey cancelled");
      case "InvalidStateError":
        throw new Error("Passkey already registered");
      case "SecurityError":
        throw new Error("Passkey is unavailable on this site");
    }
  }
  throw reason;
}

export async function signInWithPasskey() {
  const options = await api.post<PublicKeyCredentialRequestOptionsJSON>("/api/auth/passkey/start");
  const credential = (await navigator.credentials
    .get({
      publicKey: PublicKeyCredential.parseRequestOptionsFromJSON(options),
    })
    .catch(handlePasskeyError)) as PublicKeyCredential | null;
  if (!credential) throw new Error("Passkey cancelled");
  await api.post("/api/auth/passkey/finish", credential.toJSON());
}

export async function registerPasskey() {
  const result = await api.post<{ id: string; options: PublicKeyCredentialCreationOptionsJSON }>(
    "/api/account/passkeys/start",
  );
  const credential = (await navigator.credentials
    .create({
      publicKey: PublicKeyCredential.parseCreationOptionsFromJSON(result.options),
    })
    .catch(handlePasskeyError)) as PublicKeyCredential | null;
  if (!credential) throw new Error("Passkey cancelled");
  await api.post(`/api/account/passkeys/finish/${result.id}`, credential.toJSON());
}
