import { gameIdToPath } from "./util"
import { fs, resolveMountConfig, Overlay, InMemory } from "@zenfs/core";
import { Zip } from "@zenfs/archives";

import { retrieveMugenBase } from "./retrieve-base";

export async function prepareFileSystem(
  gameId: string, mugenBaseUrl: string
){
  const rootPath = gameIdToPath(gameId);
  if(fs.existsSync(rootPath)){
    throw new Error("This game has already been configured")
  }

  const mugenBase = await retrieveMugenBase(mugenBaseUrl);

  const [zipFs, memFs] = await Promise.all([
    resolveMountConfig({
      backend: Zip,
      data: mugenBase
    }),
    resolveMountConfig({
      backend: InMemory
    })
  ])

  const overlayFs = await resolveMountConfig({
    backend: Overlay,
    readable: zipFs,
    writable: memFs
  })

  if(fs.existsSync(rootPath)){
    throw new Error("This game has already been configured")
  }

  fs.mount(rootPath, overlayFs);
}

export function destroyFileSystem(gameId: string){
  const rootPath = gameIdToPath(gameId);
  if(!fs.existsSync(rootPath)){
    throw new Error("This game has not been configured")
  }
  fs.umount(rootPath);
}
