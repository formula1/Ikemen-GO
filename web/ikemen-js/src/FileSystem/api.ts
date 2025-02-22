import { fs } from "@zenfs/core";
import { join as pathJoin, sep as pathSeperator } from "node:path";

import { FileSystemState, IFileSystemAPI } from "./types";
import { prepareFileSystem, destroyFileSystem } from "./life-cycle";

export class FileSystemAPI implements IFileSystemAPI {
  public state: FileSystemState = FileSystemState.preparing;
  public prepPromise: Promise<FileSystemAPI>
  constructor(private rootPath: string, private mugenBaseURL: string){
    this.prepPromise = Promise.resolve().then(async ()=>{
      try {
        await prepareFileSystem(this.rootPath, this.mugenBaseURL);
        if(this.state as FileSystemState === FileSystemState.destroyed){
          await destroyFileSystem(this.rootPath);
          throw new Error("This has been destroyed");
        }
        this.state = FileSystemState.ready;
        return this;
      }catch(e){
        this.state = FileSystemState.failed;
        throw e;
      }
    })
  }

  async waitTillReady(): Promise<FileSystemAPI>{
    return this.prepPromise;
  }

  destroy(){
    switch(this.state){
      case FileSystemState.nothing:
        throw new Error("This filesystem has not been prepared");
      case FileSystemState.preparing:
        this.state = FileSystemState.destroyed;
        break;
      case FileSystemState.ready:
        destroyFileSystem(this.rootPath);
        this.state = FileSystemState.destroyed;
        break;
      case FileSystemState.failed:
        this.state = FileSystemState.destroyed;
        break;
      case FileSystemState.destroyed:
        throw new Error("This filesystem has already been destroyed");
    }
  }

  readFile(path: string){
    path = pathJoin(this.rootPath, path);
    if(!fs.existsSync(path)) return null;
    return fs.readFileSync(path);
  }
  stat(path: string){
    path = pathJoin(this.rootPath, path);
    if(!fs.existsSync(path)) return null;
    const stat = fs.statSync(path);
    return {
      name: path,
      size: stat.size,
      dir: stat.isDirectory()
    };
  }
  readDir(path: string){
    path = pathJoin(this.rootPath, path);
    if(!fs.existsSync(path)) return null;
    return fs.readdirSync(path);
  }
  exists(path: string){
    path = pathJoin(this.rootPath, path);
    return fs.existsSync(path);
  }

  // Write operations
  writeFile(path: string, data: Uint8Array){
    path = pathJoin(this.rootPath, path);
    fs.writeFileSync(path, data);
  }

  mkdir(path: string){
    path = pathJoin(this.rootPath, path);
    fs.mkdirSync(path);
  }

  mkdirAll(originalPath: string){
    const path = pathJoin(this.rootPath, originalPath);
    if (fs.existsSync(path)) {
      const stats = fs.statSync(path);
      if (stats.isDirectory()) {
          return;
      }
      throw new Error(`Path exists but is not a directory: ${originalPath}`);
    }
    const peices = path.split(pathSeperator);
    for(let i = 0; i < peices.length; i++){
      const peice = peices[i];
      const partialPath = peices.slice(0, i + 1).join(pathSeperator);
      if(!fs.existsSync(partialPath)){
        fs.mkdirSync(partialPath);
        continue;
      }
      if(!fs.statSync(partialPath).isDirectory()){
        throw new Error(`Path exists but is not a directory: ${originalPath}`);
      }
    }
  }
  remove(path: string){
    path = pathJoin(this.rootPath, path);
    fs.unlinkSync(path);
  }
}
