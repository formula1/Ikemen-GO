const mugenBases = new Map<string, ArrayBuffer>()
const activeFetches = new Map<string, Promise<ArrayBuffer>>()

export function retrieveMugenBase(mugenBaseUrl: string){
  const url = (new URL(mugenBaseUrl)).href;

  const found = mugenBases.get(url);
  if(found) return found;

  const active = activeFetches.get(url);
  if(active) return active

  const retrievePromise = Promise.resolve().then(async ()=>{
    const response = await fetch(url);
    if(!response.ok){
      throw new Error("Failed to fetch mugen base zip");
    }
    if(response.headers.get("content-type") !== "application/zip"){
      throw new Error("Mugen base is not a zip file");
    }
    const buffer = await response.arrayBuffer();
    mugenBases.set(url, buffer);
    activeFetches.delete(url);

    return buffer;
  });

  activeFetches.set(url, retrievePromise);

  return retrievePromise;
}
