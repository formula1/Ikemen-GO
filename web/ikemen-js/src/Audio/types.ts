import { FileSystemAPI } from "../FileSystem";

export interface IAudioManager {
  fs: FileSystemAPI;
  audioContext: AudioContext;
  bgm: IBackgroundMusic | null;
  channels: Map<string, SoundChannelState>;
  masterGain: GainNode;
}

export interface AudioSource {
  id: string;
  buffer: AudioBuffer;
  node: AudioBufferSourceNode;
  gainNode: GainNode;
  panNode: StereoPannerNode;
}

export interface IBackgroundMusic {
  paused: boolean;
  stop: ()=>void;
  pause: ()=>void;
  resume: ()=>void;
  setVolume: (volume: number)=>void;
}

export interface SoundChannelState {
  source: AudioSource;
  group: number;
  number: number;
  volume: number;
  pan: number;
}
