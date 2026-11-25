type Sources = { flv?: string; hls?: string }
export function playRoom(video: HTMLVideoElement, room: string, report: (text: string) => void, source: (mode: string, session?: string) => void) {
  const scope = new AbortController()
  let release = () => {}
  let attempts = 0
  let generation = 0
  let timer: ReturnType<typeof setTimeout>
  async function connect() {
    const run = ++generation
    report('正在获取直播源…')
    try {
      const response = await fetch('/api/rooms/' + encodeURIComponent(room) + '/stream', {
        signal: AbortSignal.any([scope.signal, AbortSignal.timeout(8000)])
      })
      if (!response.ok) throw Error()
      const {mode, session, live, sources}: {mode: string; session?: string; live: boolean; sources: Sources} = await response.json()
      if (scope.signal.aborted) return
      source(mode, live ? session : undefined)
      if (!live) {
        report(mode === 'none' ? '本地弹幕' : '直播间未开播，30 秒后刷新')
        timer = setTimeout(connect, 30000)
        return
      }
      async function attempt(flv: boolean) {
        const id = ++generation
        release()
        release = () => {}
        let startup: ReturnType<typeof setTimeout>
        const failed = () => {
          if (id !== generation || scope.signal.aborted) return
          clearTimeout(startup)
          if (flv && sources.hls) void attempt(false)
          else retry()
        }
        const playing = () => {
          clearTimeout(startup)
          attempts = 0
          report(mode === 'obs' ? 'OBS 直播中 · HLS' : flv ? '直播中 · FLV' : '直播中 · HLS')
        }
        const waiting = () => {
          clearTimeout(startup)
          if (!video.paused) startup = setTimeout(failed, 12000)
        }
        const clean = () => {
          clearTimeout(startup)
          video.removeEventListener('waiting', waiting)
          video.removeEventListener('playing', playing)
          video.removeEventListener('error', failed)
          video.removeEventListener('ended', failed)
        }
        release = clean
        video.addEventListener('waiting', waiting)
        video.addEventListener('playing', playing)
        video.addEventListener('error', failed)
        video.addEventListener('ended', failed)
        startup = setTimeout(failed, 12000)
        try {
          if (flv) {
            const {default: mpegts} = await import('mpegts.js')
            if (id !== generation) return
            if (!mpegts.getFeatureList().mseLivePlayback) throw Error()
            const player = mpegts.createPlayer({type:'flv', isLive:true, url:sources.flv!}, {liveBufferLatencyChasing:true})
            release = () => { clean(); player.destroy() }
            player.on(mpegts.Events.ERROR, failed)
            player.attachMediaElement(video)
            player.load()
          } else if (sources.hls && video.canPlayType('application/vnd.apple.mpegurl')) {
            video.src = sources.hls
          } else {
            const {default: Hls} = await import('hls.js')
            if (id !== generation) return
            if (!sources.hls || !Hls.isSupported()) throw Error()
            const player = new Hls()
            release = () => { clean(); player.destroy() }
            player.on(Hls.Events.ERROR, (_, data) => { if (data.fatal) failed() })
            player.loadSource(sources.hls)
            player.attachMedia(video)
          }
          await video.play()
        } catch (e) {
          if (id !== generation) return
          if (e instanceof DOMException && e.name === 'NotAllowedError') {
            clearTimeout(startup)
            report('点击视频播放')
          } else failed()
        }
      }
      void attempt(Boolean(sources.flv))
    } catch {
      if (!scope.signal.aborted && run === generation) retry()
    }
  }
  function retry() {
    generation++
    release()
    release = () => {}
    report('直播源暂不可用，正在重连…')
    timer = setTimeout(connect, Math.min(30000, 1000 * 2 ** Math.min(attempts++, 5)))
  }
  void connect()
  return () => {
    generation++
    scope.abort()
    clearTimeout(timer)
    release()
    video.removeAttribute('src')
    video.load()
    report('')
  }
}
