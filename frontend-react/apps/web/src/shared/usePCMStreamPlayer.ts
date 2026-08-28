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
 * 播放服务端持续推送的 PCM16 单声道音频块，并通过分析器逐帧回传实际播放振幅，驱动嘴型同步。
 */
export function usePCMStreamPlayer(options: PCMStreamPlayerOptions = {}): PCMStreamPlayerController {
  const audioContextRef = useRef<AudioContext | null>(null)
  const analyserRef = useRef<AnalyserNode | null>(null)
  const analyserFrameRef = useRef<number | null>(null)
  const nextStartTimeRef = useRef(0)
  const activeSourcesRef = useRef<Set<AudioBufferSourceNode>>(new Set())
  const playbackEndResolversRef = useRef<Array<() => void>>([])

  /**
   * 确保存在可用的 Web Audio 上下文与分析器，并在浏览器挂起后恢复到可播放状态。
   * 所有音频块统一经分析器汇入扬声器，保证回传振幅与实际听到的时间线一致。
   */
  async function ensureAudioContext(): Promise<AudioContext> {
    if (!audioContextRef.current) {
      const audioContext = new AudioContext()
      const analyser = audioContext.createAnalyser()
      analyser.fftSize = 2048
      analyser.connect(audioContext.destination)
      audioContextRef.current = audioContext
      analyserRef.current = analyser
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
   * 逐帧读取分析器振幅并回传，与陪伴页 TTS 的嘴型驱动同源；音频全部播完后自动归零并停止循环。
   */
  function startLevelLoop(): void {
    if (analyserFrameRef.current !== null) {
      return
    }

    function syncLevelFromAnalyser(): void {
      if (!analyserRef.current || !audioContextRef.current) {
        analyserFrameRef.current = null
        return
      }

      if (activeSourcesRef.current.size === 0) {
        analyserFrameRef.current = null
        options.onLevelChange?.(0)
        return
      }

      const analyserNode = analyserRef.current
      const buffer = new Uint8Array(analyserNode.fftSize)
      analyserNode.getByteTimeDomainData(buffer)
      let sum = 0
      for (const value of buffer) {
        sum += Math.abs(value - 128)
      }
      options.onLevelChange?.(Math.min(sum / buffer.length / 26, 1))
      analyserFrameRef.current = window.requestAnimationFrame(syncLevelFromAnalyser)
    }

    analyserFrameRef.current = window.requestAnimationFrame(syncLevelFromAnalyser)
  }

  /**
   * 停止振幅回传循环，配合调用方把嘴型收敛回闭合状态。
   */
  function stopLevelLoop(): void {
    if (analyserFrameRef.current !== null) {
      window.cancelAnimationFrame(analyserFrameRef.current)
      analyserFrameRef.current = null
    }
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
    const analyser = analyserRef.current
    if (!analyser) {
      return
    }

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
    for (let index = 0; index < int16Data.length; index += 1) {
      float32Data[index] = Math.max(-1, Math.min(1, int16Data[index] / 0x8000))
    }

    const targetSampleRate = sampleRate > 0 ? sampleRate : 24000
    const audioBuffer = audioContext.createBuffer(1, float32Data.length, targetSampleRate)
    audioBuffer.copyToChannel(float32Data, 0)

    const source = audioContext.createBufferSource()
    source.buffer = audioBuffer
    source.connect(analyser)
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
    startLevelLoop()
  }

  /**
   * 立即停止所有已排队或正在播放的音频块，释放上下文并重置内部时间线与嘴型开合值。
   */
  function stop(): void {
    stopLevelLoop()

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

    options.onLevelChange?.(0)

    if (audioContextRef.current) {
      void audioContextRef.current.close()
      audioContextRef.current = null
      analyserRef.current = null
    }
  }

  /**
   * 判断当前是否仍有排队或播放中的音频块。
   */
  function isPlaying(): boolean {
    return activeSourcesRef.current.size > 0
  }

  /**
   * 等待全部音频块播完；没有活跃块时立即返回。
   */
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
