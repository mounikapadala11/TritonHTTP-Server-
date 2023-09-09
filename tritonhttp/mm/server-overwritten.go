package tritonhttp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	// Addr specifies the TCP address for the server to listen on,
	// in the form "host:port". It shall be passed to net.Listen()
	// during ListenAndServe().
	Addr string // e.g. ":0"

	// VirtualHosts contains a mapping from host name to the docRoot path
	// (i.e. the path to the directory to serve static files from) for
	// all virtual hosts that this server supports
	VirtualHosts map[string]string
}

// ListenAndServe listens on the TCP network address s.Addr and then
// handles requests on incoming connections.
func (s *Server) ListenAndServe() error {

	if err:=s.ValidateServerSetup(); err!=nil {
		return err
	 }

	//listening
	listen,err:=net.Listen("tcp",s.Addr)
	if err!=nil{
		return err
	}
	defer listen.Close()
		
	// fmt.Println("Listening On", ln.Addr())

	for {
		conn, err := listen.Accept()
		if err != nil {
		     return err
		}
		fmt.Println("accepted the connection", conn.RemoteAddr())
		go s.HandleConnection(conn)
	}
	// panic("todo")
}


func (s *Server) ValidateServerSetup() error {

	for host, docRoot := range s.VirtualHosts {
		fi, err := os.Stat(docRoot)
		if os.IsNotExist(err) {
			return fmt.Errorf("doc root for host %s does not exist: %v", host, err)
		}

		if !fi.IsDir() {
			return fmt.Errorf("doc root for host %s is not a directory: %q", host, docRoot)
		}
	}

	return nil
}


func (s *Server) HandleConnection(conn net.Conn) {
	br := bufio.NewReader(conn)
	for {
		// Setting timeout
		if err0 := conn.SetReadDeadline(time.Now().Add(CONNECT_TIMEOUT)); err0 != nil {
			log.Printf("Failed to set timeout for connection %v", conn)
			_ = conn.Close()
			return
		}

		res:=&Response{}
		// Read next request from the client
		req, any, ReadErr := ReadRequest(br,res,conn)

        // fmt.Println("Request",req)

		//timer checked
		
		//Handle EOF
		if errors.Is(ReadErr, io.EOF) {
			log.Printf("Connection closed due to EOF error by %v", conn.RemoteAddr())
			_ = conn.Close()
			return
		}

		//Handling timeout
		if err, ok := ReadErr.(net.Error); ok && err.Timeout() {
			if !any {
				log.Printf("Connection to %v timed out", conn.RemoteAddr())
				_ = conn.Close()
				return
			}
			res.Handle400Request(conn)
			_=conn.Close()
			return
		}    

	
		
		if(ReadErr)!=nil{
			log.Printf("Handle bad request for error: %v", ReadErr)
			res.Handle400Request(conn)
			_ = conn.Close()
			return
		}

		//checking between 200 and 404
		DocRoot, ok := s.VirtualHosts[req.Host]
		if !ok {
			// If the Host header is not in the virtual hosts map, this is 400
			fmt.Println("entered beacuse no valid docroot found for virtual host")
			res.Handle400Request(conn)
			//_ = conn.Close()
		} else{
	
		req.URL=validUrl(req.URL,conn)

        fmt.Println("req",req)
		
	
		absPath:=filepath.Join(DocRoot, req.URL)
		
		res.Request=req
		if absPath[:len(DocRoot)] != DocRoot {
			res.HandleBadRequest(req,conn)
		} else if _, err := os.Stat(absPath); errors.Is(err, os.ErrNotExist) {
			res.HandleBadRequest(req,conn)
			// fmt.Println("404 Not Found")
		} else{
			res.giveRespOK(req,DocRoot,conn)
			fmt.Println("response object******:",res)
			// return			
		}
	}

		if(req.Close){
			_=conn.Close()
			return
		}
	
	}    

}


func (res *Response) HandleBadRequest(req *Request,conn net.Conn) {
	fmt.Println("404 error entered")
	
	
	res.Proto = "HTTP/1.1"
	res.StatusCode = 404
	res.StatusText="Not Found"
	// res.FilePath = ""

	res.Headers=make(map[string]string)
	t := time.Now()
	d:=FormatTime(t)
	res.Headers[CanonicalHeaderKey("Date")]=d
	if req.Close{
		res.Headers["Connection"]="close"
	}

	err0 := res.Write(conn,"")
	if err0 != nil {
			fmt.Println(err0)
	}
		// conn.Close()
}

func (res *Response)giveRespOK(req *Request,DocRoot string,conn net.Conn){
	fmt.Println("200 entered")
	
	res.Proto="HTTP/1.1"
	res.StatusCode=200
	res.StatusText="Ok"
	res.FilePath=filepath.Join(DocRoot, req.URL)
	// res.Request=req

	res.Headers=make(map[string]string)
    body:=res.headersResponse(req)
    // _, ok := req.Headers["Connection"]
	// if !ok{
	// 	  res.Headers["Connection"]="Close"
	// 	  req.Close=true
	// } 
	if req.Close{
		res.Headers["Connection"]="close"
	}

	err0 := res.Write(conn,body)
	if err0 != nil {
			fmt.Println(err0)
	}
}

func (res *Response)headersResponse(req *Request) (string){

	t := time.Now()
	d:=FormatTime(t)
	res.Headers[CanonicalHeaderKey("Date")]=d

	data, err := os.Stat(res.FilePath)
	if errors.Is(err, os.ErrNotExist) {
		log.Print(err)
	}

	body, err := os.ReadFile(res.FilePath)
	if  err != nil{
			return fmt.Sprintf("%v",err)
	}
		
	file:=req.URL
	idx := strings.Index(file,".")
	// words:=strings.Split(file,"/")
	// fmt.Println(words)
	// a,b,c:=ContentLen(words[len(words)-1])

	// ct:=FormatTime(c)
   fmt.Println(file[idx:])

	// res.Headers=make(map[string]string)
	res.Headers[CanonicalHeaderKey("Content-Length")]=strconv.Itoa(len(body))
	res.Headers[CanonicalHeaderKey("Content-Type")]=MIMETypeByExtension(file[idx:])//???
	res.Headers[CanonicalHeaderKey("Last-Modified")]=FormatTime(data.ModTime())
	return string(body)

	
}


func (res *Response) Handle400Request(conn net.Conn) {
	fmt.Println("400 error entered")
	res.Proto = "HTTP/1.1"
	res.StatusCode = 400
	res.StatusText="Bad Request"
	res.FilePath = ""
	res.Request=nil

	res.Headers=make(map[string]string)
	res.Headers[CanonicalHeaderKey("Date")]=FormatTime(time.Now())
	res.Headers["Connection"]="Close"

	err0 := res.Write(conn,"")
	if err0 != nil {
			fmt.Println(err0)
	}

}



//---------------------------
func validUrl(URL string,conn net.Conn) string {
	// fmt.Println("validating URL")
	
	if URL[len(URL)-1] == '/'{
		temp:=URL+"index.html"
		URL=temp
		// fmt.Println(URL)
		return URL
	}

	return URL
}
//--------------------------------------------------------------------

func ReadRequest(br *bufio.Reader,res *Response,conn net.Conn) (req *Request,any bool, err error) {
	flag:=0
	HostHeaderExist:=false
	req = &Request{}
	for{
		line, err := ReadLine(br)
		//fmt.Printf(line)
		if err != nil {
			return nil, true, err
		}
		if line == "" {
			// This marks header end
			break
		}

		//for rest of the headers
		if (flag!=0){
			HostHeaderExist=true
			readHeader(line,req,conn)
		}

		//for First Line
		if (flag==0){
			flag=flag+1
			req.Method, req.URL, req.Proto, err = parseRequestLine(line,res,conn)
				if err != nil {
					fmt.Println("entering error of parsereq bad line",badStringError("malformed start line", line))
					return nil,true, badStringError("malformed start line", line)
				}

				if !validStatusLine(req) {
					return nil,true, badStringError("invalid Status Line",req.Method)
				}
		}



		
	}

	if !HostHeaderExist {
		//res.HandleInvalidRequest(conn)
		return nil, true, fmt.Errorf("400")
	}

    return req,true,nil 
}

func validStatusLine(req *Request) bool {
	
	return (req.Method == "GET" || req.Proto =="HTTP/1.1" ||req.URL[0] == '/')

}

func readHeader(line string,req *Request,conn net.Conn){
	
	res:=&Response{}
    req.Headers = make(map[string]string)
	//splitting
	headkp:= strings.SplitN(line, ":", 2)
	fmt.Println("headers singles",headkp)
	if len(headkp) != 2 {
	    // 400.....
		fmt.Println("entering 400 beacause bad header")
		res.Handle400Request(conn)
		return 
	}
	headkp[0]=strings.TrimSpace(headkp[0])
	headkp[1]=strings.TrimSpace(headkp[1])
	// fmt.Println("headkp::",headkp[0],headkp[1])
	if(headkp[0]=="Host"){
		req.Host=headkp[1]
		
	} else if(headkp[0]=="Connection" && headkp[1]=="close"){
		req.Close=true
	} else {
		req.Headers[headkp[0]]=headkp[1]
	}

	
}


func ReadLine(br *bufio.Reader) (string, error) {
	var line string
	for {
		temp, err := br.ReadString('\n')
		line += temp
		// Return the error
		if err != nil {
			return line, err
		}
		// Return the line when reaching line end
		if strings.HasSuffix(line, "\r\n") {
			line = line[:len(line)-2]
			return line, nil
		}
	}
}

func badStringError(what, val string) error {
	return fmt.Errorf("%s %q", what, val)
}
func parseRequestLine(line string,res *Response,conn net.Conn) (string,string,string, error) {
	// fmt.Println("parsereqline",line)
	fields := strings.SplitN(line, " ", 3)
	if len(fields) != 3 {
		//res.HandleInvalidRequest(conn)
		return "", "", "", fmt.Errorf("could not parse the request line, got fields %v", fields)
	}
	// fmt.Println(fields[0], fields[1], fields[2])
	return fields[0], fields[1], fields[2], nil
}
//---------------------------------
func (res *Response) Write(conn net.Conn,body string) error {
	// bw := bufio.NewWriter(w) 
	

	//statusLine
	first:=fmt.Sprintf("%v %v %v\r\n", res.Proto, res.StatusCode, res.StatusText)
	
	//<key><colon>(<space>*)<value><CRLF>
	head:=""
	//-----------------
	//Sort keys in Headers map to print out response headers in canonical order
	keys:=make([]string,0)
	for k := range res.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	// fmt.Println("start key",keys)
	for _, k := range keys {
		head = head + k + ": " + res.Headers[k] + "\r\n"
	}
	//----------------

	output:= first+head+"\r\n"


	// var content []byte
	// var err error
	if(res.StatusCode==200){
    //  fmt.Println("body:",body,"length:",len(body))
	// if content, err = os.ReadFile(res.FilePath); err != nil{
	// 	return err
	// }
	b:=body
	fmt.Println(b)
    output=output+b
   }

	if _, err := conn.Write([]byte(output)); err != nil {
	// .WriteString(output)
		return err
	}
	// if _, err := w.Write(content); err != nil{
	// 	return err
	// }

	// if err := bw.Flush(); err != nil {
	// 	return err
	// }
	return nil
}