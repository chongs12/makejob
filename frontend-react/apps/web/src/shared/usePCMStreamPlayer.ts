import { useEffect, useRef } from 'react'

export interface PCMStreamPlayerOptions {
  onLevelChange?: (level: number) => void
}

export interface PCMStreamPlayerController {
  enqueuePCM16Base64: (audioBase64: string, sampleRate: number) => Promise<void>
  preparePlayback: () => Promise<void>
  stop: () => void
  isPlaying: () => boolean
  waitForPlaybackEnd: () => Promise<void>
}

/**
 * 播放服务端持续推送的 PCM16 单声道音频块，并把粗略振幅回传给调用方驱动嘴型。
 */
export function usePCMStreamPlayer(options: PCMStreamPlayerOptions = {}): PCMStreamPlayerController {
  const audioContextRef = useRef<AudioContext | null>(null)
  const nextStartTimeRef = useRef(0)
  const activeSourcesRef = useRef<Set<AudioBufferSourceNode>>(new Set())
  const levelResetTimerRef = useRef<number | null>(null)
  const playbackEndResolversRef = useRef<Array<() => void>>([])

  /**
   * 确保存在可用的 Web Audio 上下文，并在浏览器挂起后恢复到可播放状态。
   */
  async function ensureAudioContext(): Promise<AudioContext> {
    if (!audioContextRef.current) {
      audioContextRef.current = new AudioContext()
    }
    if (audioContextRef.current.state === 'suspended') {
      await audioContextRef.current.resume()
    }
    return audioContextRef.current
  }

  /**
   * 提前创建并唤醒播放上下文，尽量把浏览器的自动播放限制暴露在真正播报前。
   */
  async function preparePlayback(): Promise<void> {
    await ensureAudioContext()
  }

  /**
   * 将一段 base64 编码的 PCM16 音频块解码、排队并顺序接到当前播放时间线上。
   */
  async function enqueuePCM16Base64(audioBase64: string, sampleRate: number): Promise<void> {
    const normalizedBase64 = audioBase64.trim()
    if (!normalizedBase64) {
      return
    }

    const audioContext = await ensureAudioContext()
    const binary = window.atob(normalizedBase64)
    const bytes = new Uint8Array(binary.length)
    for (let index = 0; index < binary.length; index += 1) {
      bytes[index] = binary.charCodeAt(index)
    }

    const int16Data = new Int16Array(bytes.buffer, bytes.byteOffset, Math.floor(bytes.byteLength / 2))
    if (int16Data.length === 0) {
      return
    }

    const float32Data = new Float32Array(int16Data.length)
    let levelSum = 0
    for (let index = 0; index < int16Data.length; index += 1) {
      const normalized = Math.max(-1, Math.min(1, int16Data[index] / 0x8000))
      float32Data[index] = normalized
      levelSum += Math.abs(normalized)
    }

    const averageLevel = Math.min(levelSum / int16Data.length * 1.8, 1)
    options.onLevelChange?.(averageLevel)
    if (levelResetTimerRef.current !== null) {
      window.clearTimeout(levelResetTimerRef.current)
      levelResetTimerRef.current = null
    }

    const targetSampleRate = sampleRate > 0 ? sampleRate : 24000
    const audioBuffer = audioContext.createBuffer(1, float32Data.length, targetSampleRate)
    audioBuffer.copyToChannel(float32Data, 0)

    const source = audioContext.createBufferSource()
    source.buffer = audioBuffer
    source.connect(audioContext.destination)
    source.onended = () => {
      activeSourcesRef.current.delete(source)
      if (activeSourcesRef.current.size === 0) {
        const resolvers = playbackEndResolversRef.current.splice(0)
        for (const resolve of resolvers) {
          resolve()
        }
      }
    }

    const startAt = Math.max(nextStartTimeRef.current, audioContext.currentTime + 0.01)
    source.start(startAt)
    nextStartTimeRef.current = startAt + audioBuffer.duration
    activeSourcesRef.current.add(source)

    levelResetTimerRef.current = window.setTimeout(() => {
      levelResetTimerRef.current = null
      options.onLevelChange?.(0)
    }, Math.max(Math.round(audioBuffer.duration * 1000), 40))
  }

  /**
   * 立即停止所有已排队或正在播放的音频块，并重置内部时间线与嘴型开合值。
   */
  function stop(): void {
    for (const source of activeSourcesRef.current) {
      try {
        source.stop()
      } catch {
        // ignore stop errors from already-finished sources
      }
    }
    activeSourcesRef.current.clear()
    nextStartTimeRef.current = 0

    // resolve any pending waitForPlaybackEnd promises
    const resolvers = playbackEndResolversRef.current.splice(0)
    for (const resolve of resolvers) {
      resolve()
    }

    if (levelResetTimerRef.current !== null) {
      window.clearTimeout(levelResetTimerRef.current)
      levelResetTimerRef.current = null
    }
    options.onLevelChange?.(0)

    if (audioContextRef.current) {
      void audioContextRef.current.close()
      audioContextRef.current = null
    }
  }

  function isPlaying(): boolean {
    return activeSourcesRef.current.size > 0
  }

  function waitForPlaybackEnd(): Promise<void> {
    if (activeSourcesRef.current.size === 0) {
      return Promise.resolve()
    }
    return new Promise<void>((resolve) => {
      playbackEndResolversRef.current.push(resolve)
    })
  }

  useEffect(() => {
    return () => {
      stop()
    }
  }, [])

  return {
    enqueuePCM16Base64,
    preparePlayback,
    stop,
    isPlaying,
    waitForPlaybackEnd,
  }
}
