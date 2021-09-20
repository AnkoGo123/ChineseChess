/**
 * 中国象棋
 * Designed by wqh, Version: 1.0
 * Copyright (C) 2020 www.wangqianhong.com
 * GUI图形界面
 */

package chess

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"time"

	"github.com/jageros/eventhub"

	"github.com/golang/freetype/truetype"
	"github.com/hajimehoshi/ebiten/examples/resources/fonts"
	"github.com/hajimehoshi/ebiten/inpututil"
	"github.com/hajimehoshi/ebiten/text"
	"golang.org/x/image/font"

	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/audio"
	"github.com/hajimehoshi/ebiten/audio/wav"
)

const aiMoveDone = 0

var SubscribeMap map[int]int

//Game 象棋窗口
type Game struct {
	sqSelected     int                   //选中的格子
	mvLast         int                   //上一步棋
	isFlipped      bool                  //是否翻转棋盘
	isGameOver     bool                  //是否游戏结束
	showMsg        string                //显示内容
	images         map[int]*ebiten.Image //图片资源
	audios         map[int]*audio.Player //音效
	audioContext   *audio.Context        //音效控制器
	singlePosition *PositionStruct       //棋局单例
}

//NewGame 创建象棋程序
func NewGame() bool {
	game := &Game{
		images:         make(map[int]*ebiten.Image),
		audios:         make(map[int]*audio.Player),
		singlePosition: nil,
	}
	game.singlePosition = NewPositionStruct(game)

	//根本不可能为nil
	//if game == nil || game.singlePosition == nil {
	//	return false
	//}

	var err error
	//音效器
	game.audioContext, err = audio.NewContext(48000)
	if err != nil {
		fmt.Print(err)
		return false
	}

	//加载资源
	if ok := game.loadResource(); !ok {
		return false
	}

	//加载开局库
	game.singlePosition.loadBook()
	game.singlePosition.startup()

	//设置窗口，接收信息
	ebiten.SetWindowSize(BoardWidth, BoardHeight)
	ebiten.SetWindowTitle("中国象棋")
	if err := ebiten.RunGame(game); err != nil {
		fmt.Print(err)
		return false
	}

	return true
}

//Update 更新状态，1秒60帧
func (g *Game) Update(screen *ebiten.Image) error {

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if g.isGameOver {
			g.isGameOver = false
			g.showMsg = ""
			g.sqSelected = 0
			g.mvLast = 0
			g.singlePosition.startup()
		} else {
			x, y := ebiten.CursorPosition()
			x = Left + (x-BoardEdge)/SquareSize
			y = Top + (y-BoardEdge)/SquareSize
			g.clickSquare(screen, squareXY(x, y))
		}
	}
	g.drawBoard(screen)
	return nil
}

//Layout 布局采用外部尺寸（例如，窗口尺寸）并返回（逻辑）屏幕尺寸，如果不使用外部尺寸，只需返回固定尺寸即可。
func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return BoardWidth, BoardHeight
}

//loadResource 加载资源
func (g *Game) loadResource() bool {
	for k, v := range resMap {
		if k >= MusicSelect {
			//加载音效
			d, err := wav.Decode(g.audioContext, audio.BytesReadSeekCloser(v))
			if err != nil {
				fmt.Print(err)
				return false
			}
			player, err := audio.NewPlayer(g.audioContext, d)
			if err != nil {
				fmt.Print(err)
				return false
			}
			g.audios[k] = player
		} else {
			//加载图片
			img, _, err := image.Decode(bytes.NewReader(v))
			if err != nil {
				fmt.Print(err)
				return false
			}
			ebitenImage, _ := ebiten.NewImageFromImage(img, ebiten.FilterDefault)
			g.images[k] = ebitenImage
		}
	}

	return true
}

//drawBoard 绘制棋盘
func (g *Game) drawBoard(screen *ebiten.Image) {
	//棋盘
	if v, ok := g.images[ImgChessBoard]; ok {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(0, 0)
		screen.DrawImage(v, op)
	}

	//棋子
	for x := Left; x <= Right; x++ {
		for y := Top; y <= Bottom; y++ {
			xPos, yPos := 0, 0
			if g.isFlipped {
				xPos = BoardEdge + (xFlip(x)-Left)*SquareSize
				yPos = BoardEdge + (yFlip(y)-Top)*SquareSize
			} else {
				xPos = BoardEdge + (x-Left)*SquareSize
				yPos = BoardEdge + (y-Top)*SquareSize
			}
			sq := squareXY(x, y)
			pc := g.singlePosition.ucpcSquares[sq]
			if pc != 0 {
				g.drawChess(xPos, yPos+5, screen, g.images[pc])
			}
			if sq == g.sqSelected || sq == src(g.mvLast) || sq == dst(g.mvLast) {
				g.drawChess(xPos, yPos, screen, g.images[ImgSelect])
			}
		}
	}
}

//drawChess 绘制棋子
func (g *Game) drawChess(x, y int, screen, img *ebiten.Image) {
	if img == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	screen.DrawImage(img, op)
}

//clickSquare 点击格子处理
func (g *Game) clickSquare(screen *ebiten.Image, sq int) {
	pc := 0
	if g.isFlipped {
		pc = g.singlePosition.ucpcSquares[squareFlip(sq)]
	} else {
		pc = g.singlePosition.ucpcSquares[sq]
	}
	//要播放的音乐索引
	var audioClip = -1
	if (pc & sideTag(g.singlePosition.sdPlayer)) != 0 {
		//如果点击自己的棋子，那么直接选中
		g.sqSelected = sq
		audioClip = MusicSelect
	} else if g.sqSelected != 0 && !g.isGameOver {
		//如果点击的不是自己的棋子，但有棋子选中了(一定是自己的棋子)，那么走这个棋子
		mv := move(g.sqSelected, sq)
		if g.singlePosition.legalMove(mv) {
			if g.singlePosition.makeMove(mv) {
				g.mvLast = mv
				g.sqSelected = 0
				//检查重复局面
				vlRep := g.singlePosition.repStatus(3)

				if g.singlePosition.isMate() {
					//如果分出胜负，那么播放胜负的声音，并且弹出不带声音的提示框
					audioClip = MusicGameWin
					g.showMsg = "Your Win!"
					g.isGameOver = true
				} else if vlRep > 0 {
					vlRep = g.singlePosition.repValue(vlRep)
					if vlRep > WinValue {
						audioClip = MusicGameLose
						g.showMsg = "Your Lose!"
					} else {
						if vlRep < -WinValue {
							g.showMsg = "Your Win!"
						} else {
							g.showMsg = "Your Draw!"
						}
						audioClip = MusicGameWin
					}
					g.isGameOver = true
				} else if g.singlePosition.nMoveNum > 100 {
					audioClip = MusicGameWin
					g.showMsg = "Your Draw!"
					g.isGameOver = true
				} else {
					if g.singlePosition.isJiangJun() {
						audioClip = MusicJiang
					} else {
						if g.singlePosition.captured() {
							audioClip = MusicEat
							g.singlePosition.setIrrev()
						} else {
							audioClip = MusicPut
						}
					}
					//启动协程
					go func() {
						time.Sleep(time.Duration(float64(time.Second) * 1.5))
						g.aiMove(screen)
						eventhub.Publish(aiMoveDone)
					}()
				}
			} else {
				audioClip = MusicJiang //被将军的声音
			}
		} else {
			fmt.Println("错误走法")
		}
		//如果根本就不符合走法(例如马不走日字)，那么不做任何处理
	}
	if audioClip != -1 {
		//不等于-1说明有要播放的音乐，此时播放
		g.playAudio(MusicPut)
	}

}

//playAudio 播放音效
func (g *Game) playAudio(value int) {
	if player, ok := g.audios[value]; ok {
		player.Rewind()
		player.Play()
	}
}

//aiMove AI移动
func (g *Game) aiMove(screen *ebiten.Image) {

	//AI走一步棋
	g.singlePosition.searchMain()
	g.singlePosition.makeMove(g.singlePosition.search.mvResult)
	//把AI走的棋标记出来
	g.mvLast = g.singlePosition.search.mvResult
	//检查重复局面
	vlRep := g.singlePosition.repStatus(3)

	var audioClip = -1
	if g.singlePosition.isMate() {
		//如果分出胜负，那么播放胜负的声音
		audioClip = MusicGameWin
		g.showMsg = "Your Lose!"
		g.isGameOver = true
	} else if vlRep > 0 {
		vlRep = g.singlePosition.repValue(vlRep)
		//vlRep是对玩家来说的分值
		if vlRep < -WinValue {
			audioClip = MusicGameLose
			g.showMsg = "Your Lose!"
		} else {
			if vlRep > WinValue {
				audioClip = MusicGameWin
				g.showMsg = "Your Lose!"
			} else {
				audioClip = MusicGameWin
				g.showMsg = "Your Draw!"
			}
		}
		g.isGameOver = true
	} else if g.singlePosition.nMoveNum > 100 {
		//超过100步的历史都判人赢了
		audioClip = MusicGameWin
		g.showMsg = "Your Draw!"
		g.isGameOver = true
	} else {
		//如果没有分出胜负，那么播放将军、吃子或一般走子的声音
		if g.singlePosition.inJiangJun() {
			audioClip = MusicJiang
		} else {
			if g.singlePosition.captured() {
				audioClip = MusicEat
			} else {
				audioClip = MusicPut
			}
		}
		if g.singlePosition.captured() {
			g.singlePosition.setIrrev()
		}
	}
	if audioClip != -1 {
		g.playAudio(audioClip)
	}
}

//messageBox 提示
func (g *Game) messageBox(screen *ebiten.Image) {
	fmt.Println(g.showMsg)
	tt, err := truetype.Parse(fonts.ArcadeN_ttf)
	if err != nil {
		fmt.Print(err)
		return
	}
	arcadeFont := truetype.NewFace(tt, &truetype.Options{
		Size:    16,
		DPI:     72,
		Hinting: font.HintingFull,
	})

	text.Draw(screen, g.showMsg, arcadeFont, 180, 288, color.White)
	text.Draw(screen, "Click mouse to restart", arcadeFont, 100, 320, color.White)
}
