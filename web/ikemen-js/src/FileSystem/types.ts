
export enum FileSystemState {
  preparing,
  failed,
  ready,
  destroyed
}

export interface IFileSystemAPI {
  state: FileSystemState
  waitTillReady(): Promise<IFileSystemAPI>
  destroy(): void


  readFile(path: string): null | Uint8Array
  readDir(path: string): null | string[]
  exists(path: string): boolean
  writeFile(path: string, data: Uint8Array): void
  mkdirAll(path: string): void
  remove(path: string): void
}
