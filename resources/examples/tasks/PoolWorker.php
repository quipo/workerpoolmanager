<?php
declare(ticks = 10);
	
/**
 * using a trait here to make get_class() return the name 
 * of the child class and not the parent class
 */
trait taskname_trait {
	/**
	 * Get the task name from the class name, without the Task.xxx suffix
	 * 
	 * @return string
	 */
	protected function _setTaskName() {
		return preg_replace('/^(.*)task(\..*)?$/i', '$1', get_class($this));
	}
}

/**
 * Base task, handling keep-alives via ZeroMQ PUB
 */
abstract class PoolWorker 
{
	use taskname_trait;

	/**
	 * @var array
	 */
	protected $config = array();

	/**
	 * @var string
	 */
	protected $taskname = 'PoolWorker';

	/**
	 * @var integer
	 */
	protected $pid = 0;


	/**
	 * Keep-alive channel
	 *
	 * @var ZMQSocket
	 */
	protected $keepAliveSocket = null;

	/**
	 * Last-alive timestamp
	 *
	 * @var int
	 */
	protected $lastAliveAt = 0;

	/**
	 * When to terminate the task
	 *
	 * @var boolean
	 */
	protected $terminate = false;

	/**
	 * Constructor
	 *
	 * @param array $config Configuration
	 */
	public function __construct(array $config) {
		$this->config      = $config;
		$this->taskname    = $this->_setTaskName();
		$this->pid         = posix_getpid();
		$this->lastAliveAt = time();
	}

	/**
	 * Set the cli process title to the script name, instead of "php"
	 * Available in PHP >= 5.5.0
	 *
	 * @return void
	 */
	protected function setProcessTitle() {
		if (version_compare(PHP_VERSION, '5.5.0') >= 0) {
			//cli_set_process_title(__FILE__);
			if (isset($_SERVER['argc'])) {
				cli_set_process_title($_SERVER['argv'][0]);
			} else {
				cli_set_process_title($this->taskname);
			}
		}
	}

	/**
	 * Init the worker process and keep it running
	 */
	public function run() {
		// set up keep-alive and signal handlers
		$this->setupKeepAliveSender();
		$this->setupSignalHandler();
		$this->setProcessTitle();

		// set up the task
		if (!$this->setUpTask()) {
			throw new Exception("Failed setting up task " . $this->taskname);
		}

		// run the worker until asked to terminate
		$this->runTask();
	}

	/**
	 * Optional method to set up the task
	 */
	protected function setUpTask() {

	}
	
	/**
	 * To be implemented in a child class, worker code to execute the task
	 */
	abstract protected function runTask();

	/**
	 * Set up the ZeroMQ PUB socket to send keep-alive messages 
	 */
	protected function setupKeepAliveSender() {
		if (empty($this->config['keepalive']['address'])) {
			throw new Exception("Cannot find keep-alive socket configuration");
		}
		$context = new ZMQContext();
		$this->sender = new ZMQSocket($context, ZMQ::SOCKET_PUB);
		$this->sender->connect($this->config['keepalive']['address']);
	}

	/**
	 * Signal handler, called on SIGTERM/SIGINT
	 */
	public function signalHandler($signal) {
		switch($signal) {
		case SIGTERM:
			error_log("Caught SIGTERM (".$this->pid.")");
			$this->terminate = true;
			break;
		case SIGKILL:
			error_log("Caught SIGKILL (".$this->pid.")");
			exit(1);
			break;
		case SIGINT:
			error_log("Caught SIGINT (".$this->pid.")");
			$this->terminate = true;
			break;
		}
	}

	/**
	 * Set up the signal handler to capture SIGTERM/SIGINT and terminate gracefully
	 */
	protected function setupSignalHandler() {
		pcntl_signal(SIGTERM, array($this, "signalHandler"));
		pcntl_signal(SIGINT, array($this, "signalHandler"));
	}

	/**
	 * Clean up resources (e.g. flush stats, close DB connections)
	 */
	public function __destruct() {
		error_log("CALLED CLEANUP");
		$this->sender->disconnect($this->config['keepalive']['address']);
		unset($this->sender);
	}

	/**
	 * Send keep-alive messages
	 */
	protected function setAlive() {
		$now = time();
		if ($now - $this->lastAliveAt < 2) {
			//error_log("SKIPPING SETALIVE");
			// only send keep-alive messages every 2 seconds or more,
			// no point spamming the keep-alive channel too frequently
			return;
		}
		$msg = array(
			'taskname' => $this->taskname,
			'pid'      => $this->pid,
		);
		error_log("SENDING SETALIVE: ".json_encode($msg));
		$this->sender->sendmulti(array($this->taskname, json_encode($msg)));
		$this->lastAliveAt = time();
	}
}
